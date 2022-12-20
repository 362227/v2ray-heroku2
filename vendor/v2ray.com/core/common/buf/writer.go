package buf

import (
	"io"

	"v2ray.com/core/common"
	"v2ray.com/core/common/errors"
)

// BufferToBytesWriter is a Writer that writes alloc.Buffer into underlying writer.
type BufferToBytesWriter struct {
	io.Writer
}

// NewBufferToBytesWriter returns a new BufferToBytesWriter.
func NewBufferToBytesWriter(writer io.Writer) *BufferToBytesWriter {
	return &BufferToBytesWriter{
		Writer: writer,
	}
}

// WriteMultiBuffer implements Writer. This method takes ownership of the given buffer.
func (w *BufferToBytesWriter) WriteMultiBuffer(mb MultiBuffer) error {
	defer mb.Release()

	bs := mb.ToNetBuffers()
	return common.Error2(bs.WriteTo(w.Writer))
}

// ReadFrom implements io.ReaderFrom.
func (w *BufferToBytesWriter) ReadFrom(reader io.Reader) (int64, error) {
	var sc SizeCounter
	err := Copy(NewReader(reader), w, CountSize(&sc))
	return sc.Size, err
}

// BufferedWriter is a Writer with internal buffer.
type BufferedWriter struct {
	writer   Writer
	buffer   *Buffer
	buffered bool
}

// NewBufferedWriter creates a new BufferedWriter.
func NewBufferedWriter(writer Writer) *BufferedWriter {
	return &BufferedWriter{
		writer:   writer,
		buffer:   New(),
		buffered: true,
	}
}

// WriteByte implements io.ByteWriter.
func (w *BufferedWriter) WriteByte(c byte) error {
	return common.Error2(w.Write([]byte{c}))
}

// Write implements io.Writer.
func (w *BufferedWriter) Write(b []byte) (int, error) {
	if !w.buffered {
		if writer, ok := w.writer.(io.Writer); ok {
			return writer.Write(b)
		}
	}

	totalBytes := 0
	for len(b) > 0 {
		if w.buffer == nil {
			w.buffer = New()
		}

		nBytes, err := w.buffer.Write(b)
		totalBytes += nBytes
		if err != nil {
			return totalBytes, err
		}
		if !w.buffered || w.buffer.IsFull() {
			if err := w.Flush(); err != nil {
				return totalBytes, err
			}
		}
		b = b[nBytes:]
	}

	return totalBytes, nil
}

// WriteMultiBuffer implements Writer. It takes ownership of the given MultiBuffer.
func (w *BufferedWriter) WriteMultiBuffer(b MultiBuffer) error {
	if !w.buffered {
		return w.writer.WriteMultiBuffer(b)
	}

	defer b.Release()

	for !b.IsEmpty() {
		if w.buffer == nil {
			w.buffer = New()
		}
		if err := w.buffer.AppendSupplier(ReadFrom(&b)); err != nil {
			return err
		}
		if w.buffer.IsFull() {
			if err := w.Flush(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Flush flushes buffered content into underlying writer.
func (w *BufferedWriter) Flush() error {
	if !w.buffer.IsEmpty() {
		if err := w.writer.WriteMultiBuffer(NewMultiBufferValue(w.buffer)); err != nil {
			return err
		}

		w.buffer = nil
	}
	return nil
}

// SetBuffered sets whether the internal buffer is used. If set to false, Flush() will be called to clear the buffer.
func (w *BufferedWriter) SetBuffered(f bool) error {
	w.buffered = f
	if !f {
		return w.Flush()
	}
	return nil
}

// ReadFrom implements io.ReaderFrom.
func (w *BufferedWriter) ReadFrom(reader io.Reader) (int64, error) {
	if err := w.SetBuffered(false); err != nil {
		return 0, err
	}

	var sc SizeCounter
	err := Copy(NewReader(reader), w, CountSize(&sc))
	return sc.Size, err
}

// Close implements io.Closable.
func (w *BufferedWriter) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}
	return common.Close(w.writer)
}

type seqWriter struct {
	writer io.Writer
}

func (w *seqWriter) WriteMultiBuffer(mb MultiBuffer) error {
	defer mb.Release()

	for _, b := range mb {
		if b.IsEmpty() {
			continue
		}
		if _, err := w.writer.Write(b.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

type noOpWriter byte

func (noOpWriter) WriteMultiBuffer(b MultiBuffer) error {
	b.Release()
	return nil
}

func (noOpWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (noOpWriter) ReadFrom(reader io.Reader) (int64, error) {
	b := New()
	defer b.Release()

	totalBytes := int64(0)
	for {
		err := b.Reset(ReadFrom(reader))
		totalBytes += int64(b.Len())
		if err != nil {
			if errors.Cause(err) == io.EOF {
				return totalBytes, nil
			}
			return totalBytes, err
		}
	}
}

var (
	// Discard is a Writer that swallows all contents written in.
	Discard Writer = noOpWriter(0)

	// DiscardBytes is an io.Writer that swallows all contents written in.
	DiscardBytes io.Writer = noOpWriter(0)
)
