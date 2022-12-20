package router

import (
	"context"

	"v2ray.com/core/common/net"
)

type Rule struct {
	Tag       string
	Condition Condition
}

func (r *Rule) Apply(ctx context.Context) bool {
	return r.Condition.Apply(ctx)
}

func cidrToCondition(cidr []*CIDR, source bool) (Condition, error) {
	ipv4Net := net.NewIPNetTable()
	ipv6Cond := NewAnyCondition()
	hasIpv6 := false

	for _, ip := range cidr {
		switch len(ip.Ip) {
		case net.IPv4len:
			ipv4Net.AddIP(ip.Ip, byte(ip.Prefix))
		case net.IPv6len:
			hasIpv6 = true
			matcher, err := NewCIDRMatcher(ip.Ip, ip.Prefix, source)
			if err != nil {
				return nil, err
			}
			ipv6Cond.Add(matcher)
		default:
			return nil, newError("invalid IP length").AtWarning()
		}
	}

	if !ipv4Net.IsEmpty() && hasIpv6 {
		cond := NewAnyCondition()
		cond.Add(NewIPv4Matcher(ipv4Net, source))
		cond.Add(ipv6Cond)
		return cond, nil
	} else if !ipv4Net.IsEmpty() {
		return NewIPv4Matcher(ipv4Net, source), nil
	} else {
		return ipv6Cond, nil
	}
}

func (rr *RoutingRule) BuildCondition() (Condition, error) {
	conds := NewConditionChan()

	if len(rr.Domain) > 0 {
		matcher := NewCachableDomainMatcher()
		for _, domain := range rr.Domain {
			matcher.Add(domain)
		}
		conds.Add(matcher)
	}

	if len(rr.UserEmail) > 0 {
		conds.Add(NewUserMatcher(rr.UserEmail))
	}

	if len(rr.InboundTag) > 0 {
		conds.Add(NewInboundTagMatcher(rr.InboundTag))
	}

	if rr.PortRange != nil {
		conds.Add(NewPortMatcher(*rr.PortRange))
	}

	if rr.NetworkList != nil {
		conds.Add(NewNetworkMatcher(rr.NetworkList))
	}

	if len(rr.Cidr) > 0 {
		cond, err := cidrToCondition(rr.Cidr, false)
		if err != nil {
			return nil, err
		}
		conds.Add(cond)
	}

	if len(rr.SourceCidr) > 0 {
		cond, err := cidrToCondition(rr.SourceCidr, true)
		if err != nil {
			return nil, err
		}
		conds.Add(cond)
	}

	if conds.Len() == 0 {
		return nil, newError("this rule has no effective fields").AtWarning()
	}

	return conds, nil
}
