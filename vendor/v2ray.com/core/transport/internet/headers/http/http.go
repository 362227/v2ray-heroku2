package http

//go:generate go run $GOPATH/src/v2ray.com/core/common/errors/errorgen/main.go -pkg http -path Transport,Internet,Headers,HTTP

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/serial"
)

const (
	// CRLF is the line ending in HTTP header
	CRLF = "\r\n"

	// ENDING is the double line ending between HTTP header and body.
	ENDING = CRLF + CRLF

	// max length of HTTP header. Safety precaution for DDoS attack.
	maxHeaderLength = 8192
)

var (
	ErrHeaderToLong = newError("Header too long.")
	writeCRLF       = serial.WriteString(CRLF)
)

type Reader interface {
	Read(io.Reader) (*buf.Buffer, error)
}

type Writer interface {
	Write(io.Writer) error
}

type NoOpReader struct{}

func (NoOpReader) Read(io.Reader) (*buf.Buffer, error) {
	return nil, nil
}

type NoOpWriter struct{}

func (NoOpWriter) Write(io.Writer) error {
	return nil
}

type HeaderReader struct {
}

func (*HeaderReader) Read(reader io.Reader) (*buf.Buffer, error) {
	buffer := buf.New()
	totalBytes := int32(0)
	endingDetected := false
	for totalBytes < maxHeaderLength {
		err := buffer.AppendSupplier(buf.ReadFrom(reader))
		if err != nil {
			buffer.Release()
			return nil, err
		}
		if n := bytes.Index(buffer.Bytes(), []byte(ENDING)); n != -1 {
			buffer.Advance(int32(n + len(ENDING)))
			endingDetected = true
			break
		}
		if buffer.Len() >= int32(len(ENDING)) {
			totalBytes += buffer.Len() - int32(len(ENDING))
			leftover := buffer.BytesFrom(-int32(len(ENDING)))
			buffer.Reset(func(b []byte) (int, error) {
				return copy(b, leftover), nil
			})
		}
	}
	if buffer.IsEmpty() {
		buffer.Release()
		return nil, nil
	}
	if !endingDetected {
		buffer.Release()
		return nil, ErrHeaderToLong
	}
	return buffer, nil
}

type HeaderWriter struct {
	header *buf.Buffer
}

func NewHeaderWriter(header *buf.Buffer) *HeaderWriter {
	return &HeaderWriter{
		header: header,
	}
}

func (w *HeaderWriter) Write(writer io.Writer) error {
	if w.header == nil {
		return nil
	}
	_, err := writer.Write(w.header.Bytes())
	w.header.Release()
	w.header = nil
	return err
}

type HttpConn struct {
	net.Conn

	readBuffer    *buf.Buffer
	oneTimeReader Reader
	oneTimeWriter Writer
	errorWriter   Writer
}

func NewHttpConn(conn net.Conn, reader Reader, writer Writer, errorWriter Writer) *HttpConn {
	return &HttpConn{
		Conn:          conn,
		oneTimeReader: reader,
		oneTimeWriter: writer,
		errorWriter:   errorWriter,
	}
}

func (c *HttpConn) Read(b []byte) (int, error) {
	if c.oneTimeReader != nil {
		buffer, err := c.oneTimeReader.Read(c.Conn)
		if err != nil {
			return 0, err
		}
		c.readBuffer = buffer
		c.oneTimeReader = nil
	}

	if !c.readBuffer.IsEmpty() {
		nBytes, _ := c.readBuffer.Read(b)
		if c.readBuffer.IsEmpty() {
			c.readBuffer.Release()
			c.readBuffer = nil
		}
		return nBytes, nil
	}

	return c.Conn.Read(b)
}

// Write implements io.Writer.
func (c *HttpConn) Write(b []byte) (int, error) {
	if c.oneTimeWriter != nil {
		err := c.oneTimeWriter.Write(c.Conn)
		c.oneTimeWriter = nil
		if err != nil {
			return 0, err
		}
	}

	return c.Conn.Write(b)
}

// Close implements net.Conn.Close().
func (c *HttpConn) Close() error {
	if c.oneTimeWriter != nil && c.errorWriter != nil {
		// Connection is being closed but header wasn't sent. This means the client request
		// is probably not valid. Sending back a server error header in this case.
		c.errorWriter.Write(c.Conn)
	}

	return c.Conn.Close()
}

func formResponseHeader(config *ResponseConfig) *HeaderWriter {
	header := buf.New()
	header.AppendSupplier(serial.WriteString(strings.Join([]string{config.GetFullVersion(), config.GetStatusValue().Code, config.GetStatusValue().Reason}, " ")))
	header.AppendSupplier(writeCRLF)

	headers := config.PickHeaders()
	for _, h := range headers {
		header.AppendSupplier(serial.WriteString(h))
		header.AppendSupplier(writeCRLF)
	}
	if !config.HasHeader("Date") {
		header.AppendSupplier(serial.WriteString("Date: "))
		header.AppendSupplier(serial.WriteString(time.Now().Format(http.TimeFormat)))
		header.AppendSupplier(writeCRLF)
	}
	header.AppendSupplier(writeCRLF)
	return &HeaderWriter{
		header: header,
	}
}

type HttpAuthenticator struct {
	config *Config
}

func (a HttpAuthenticator) GetClientWriter() *HeaderWriter {
	header := buf.New()
	config := a.config.Request
	header.AppendSupplier(serial.WriteString(strings.Join([]string{config.GetMethodValue(), config.PickUri(), config.GetFullVersion()}, " ")))
	header.AppendSupplier(writeCRLF)

	headers := config.PickHeaders()
	for _, h := range headers {
		header.AppendSupplier(serial.WriteString(h))
		header.AppendSupplier(writeCRLF)
	}
	header.AppendSupplier(writeCRLF)
	return &HeaderWriter{
		header: header,
	}
}

func (a HttpAuthenticator) GetServerWriter() *HeaderWriter {
	return formResponseHeader(a.config.Response)
}

func (a HttpAuthenticator) Client(conn net.Conn) net.Conn {
	if a.config.Request == nil && a.config.Response == nil {
		return conn
	}
	var reader Reader = NoOpReader{}
	if a.config.Request != nil {
		reader = new(HeaderReader)
	}

	var writer Writer = NoOpWriter{}
	if a.config.Response != nil {
		writer = a.GetClientWriter()
	}
	return NewHttpConn(conn, reader, writer, NoOpWriter{})
}

func (a HttpAuthenticator) Server(conn net.Conn) net.Conn {
	if a.config.Request == nil && a.config.Response == nil {
		return conn
	}
	return NewHttpConn(conn, new(HeaderReader), a.GetServerWriter(), formResponseHeader(&ResponseConfig{
		Version: &Version{
			Value: "1.1",
		},
		Status: &Status{
			Code:   "500",
			Reason: "Internal Server Error",
		},
		Header: []*Header{
			{
				Name:  "Connection",
				Value: []string{"close"},
			},
			{
				Name:  "Cache-Control",
				Value: []string{"private"},
			},
			{
				Name:  "Content-Length",
				Value: []string{"0"},
			},
		},
	}))
}

func NewHttpAuthenticator(ctx context.Context, config *Config) (HttpAuthenticator, error) {
	return HttpAuthenticator{
		config: config,
	}, nil
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewHttpAuthenticator(ctx, config.(*Config))
	}))
}
