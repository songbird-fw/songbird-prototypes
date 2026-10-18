package model

import "time"

// SystemSpec is system.schema.json: the singleton named "system" with the
// global settings. Optional booleans are pointers so that an explicit false
// survives a round trip.
type SystemSpec struct {
	Hostname    string       `yaml:"hostname" json:"hostname"`
	Timezone    string       `yaml:"timezone,omitempty" json:"timezone,omitempty"`
	Admin       *Admin       `yaml:"admin,omitempty" json:"admin,omitempty"`
	NTP         *NTP         `yaml:"ntp,omitempty" json:"ntp,omitempty"`
	DNS         *DNS         `yaml:"dns,omitempty" json:"dns,omitempty"`
	Logging     *Logging     `yaml:"logging,omitempty" json:"logging,omitempty"`
	Session     *Session     `yaml:"session,omitempty" json:"session,omitempty"`
	NAT64       *NAT64       `yaml:"nat64,omitempty" json:"nat64,omitempty"`
	Git         *Git         `yaml:"git,omitempty" json:"git,omitempty"`
	Updates     *Updates     `yaml:"updates,omitempty" json:"updates,omitempty"`
	ThreatFeeds *ThreatFeeds `yaml:"threatFeeds,omitempty" json:"threatFeeds,omitempty"`
}

type Admin struct {
	HTTPSPort      int          `yaml:"httpsPort,omitempty" json:"httpsPort,omitempty"`
	RedirectHTTP   *bool        `yaml:"redirectHttp,omitempty" json:"redirectHttp,omitempty"`
	SessionTimeout string       `yaml:"sessionTimeout,omitempty" json:"sessionTimeout,omitempty"`
	Lockout        *Lockout     `yaml:"lockout,omitempty" json:"lockout,omitempty"`
	TrustedHosts   []string     `yaml:"trustedHosts,omitempty" json:"trustedHosts,omitempty"`
	Certificate    *Certificate `yaml:"certificate,omitempty" json:"certificate,omitempty"`
	UI             *AdminUI     `yaml:"ui,omitempty" json:"ui,omitempty"`
}

type Lockout struct {
	Threshold int    `yaml:"threshold,omitempty" json:"threshold,omitempty"`
	Duration  string `yaml:"duration,omitempty" json:"duration,omitempty"`
}

type Certificate struct {
	Mode string `yaml:"mode" json:"mode"`
	ACME *ACME  `yaml:"acme,omitempty" json:"acme,omitempty"`
}

type ACME struct {
	Email           string   `yaml:"email,omitempty" json:"email,omitempty"`
	DirectoryURL    string   `yaml:"directoryUrl,omitempty" json:"directoryUrl,omitempty"`
	Domains         []string `yaml:"domains,omitempty" json:"domains,omitempty"`
	DNSProvider     string   `yaml:"dnsProvider,omitempty" json:"dnsProvider,omitempty"`
	DNSCredentials  string   `yaml:"dnsCredentials,omitempty" json:"dnsCredentials,omitempty"`
	RenewBeforeDays int      `yaml:"renewBeforeDays,omitempty" json:"renewBeforeDays,omitempty"`
}

type AdminUI struct {
	Theme    string `yaml:"theme,omitempty" json:"theme,omitempty"`
	Language string `yaml:"language,omitempty" json:"language,omitempty"`
}

type NTP struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	Servers           []string `yaml:"servers,omitempty" json:"servers,omitempty"`
	ServeToInterfaces []string `yaml:"serveToInterfaces,omitempty" json:"serveToInterfaces,omitempty"`
}

type DNS struct {
	Mode                  string           `yaml:"mode" json:"mode"`
	Upstreams             []string         `yaml:"upstreams,omitempty" json:"upstreams,omitempty"`
	FailoverAfterTimeouts int              `yaml:"failoverAfterTimeouts,omitempty" json:"failoverAfterTimeouts,omitempty"`
	DNSSEC                *bool            `yaml:"dnssec,omitempty" json:"dnssec,omitempty"`
	LocalDomain           string           `yaml:"localDomain,omitempty" json:"localDomain,omitempty"`
	LocalRecords          []DNSLocalRecord `yaml:"localRecords,omitempty" json:"localRecords,omitempty"`
	DNS64                 *bool            `yaml:"dns64,omitempty" json:"dns64,omitempty"`
	ServeToInterfaces     []string         `yaml:"serveToInterfaces,omitempty" json:"serveToInterfaces,omitempty"`
}

type DNSLocalRecord struct {
	Name    string `yaml:"name" json:"name"`
	Address string `yaml:"address" json:"address"`
}

type Logging struct {
	Directory         string             `yaml:"directory,omitempty" json:"directory,omitempty"`
	Rotation          *Rotation          `yaml:"rotation,omitempty" json:"rotation,omitempty"`
	AnonymizeSourceIP *AnonymizeSourceIP `yaml:"anonymizeSourceIp,omitempty" json:"anonymizeSourceIp,omitempty"`
	Syslog            *Syslog            `yaml:"syslog,omitempty" json:"syslog,omitempty"`
}

type Rotation struct {
	MaxFileSizeMb int   `yaml:"maxFileSizeMb,omitempty" json:"maxFileSizeMb,omitempty"`
	RetentionDays int   `yaml:"retentionDays,omitempty" json:"retentionDays,omitempty"`
	Compress      *bool `yaml:"compress,omitempty" json:"compress,omitempty"`
}

type AnonymizeSourceIP struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	Interfaces   []string `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
	IPv4MaskBits int      `yaml:"ipv4MaskBits,omitempty" json:"ipv4MaskBits,omitempty"`
	IPv6MaskBits int      `yaml:"ipv6MaskBits,omitempty" json:"ipv6MaskBits,omitempty"`
}

type Syslog struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Server    string `yaml:"server,omitempty" json:"server,omitempty"`
	Port      int    `yaml:"port,omitempty" json:"port,omitempty"`
	Transport string `yaml:"transport,omitempty" json:"transport,omitempty"`
	Facility  string `yaml:"facility,omitempty" json:"facility,omitempty"`
}

type Session struct {
	Timeouts                *SessionTimeouts `yaml:"timeouts,omitempty" json:"timeouts,omitempty"`
	MaxSessions             int              `yaml:"maxSessions,omitempty" json:"maxSessions,omitempty"`
	IPv4NonInitialFragments string           `yaml:"ipv4NonInitialFragments,omitempty" json:"ipv4NonInitialFragments,omitempty"`
	OnRuleChange            string           `yaml:"onRuleChange,omitempty" json:"onRuleChange,omitempty"`
}

type SessionTimeouts struct {
	TCPEstablished   string `yaml:"tcpEstablished,omitempty" json:"tcpEstablished,omitempty"`
	TCPHandshake     string `yaml:"tcpHandshake,omitempty" json:"tcpHandshake,omitempty"`
	TCPClosing       string `yaml:"tcpClosing,omitempty" json:"tcpClosing,omitempty"`
	UDP              string `yaml:"udp,omitempty" json:"udp,omitempty"`
	UDPBidirectional string `yaml:"udpBidirectional,omitempty" json:"udpBidirectional,omitempty"`
	ICMP             string `yaml:"icmp,omitempty" json:"icmp,omitempty"`
}

type NAT64 struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Prefix  string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
}

type Git struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	Remote        string   `yaml:"remote,omitempty" json:"remote,omitempty"`
	Branch        string   `yaml:"branch,omitempty" json:"branch,omitempty"`
	AutoPush      *bool    `yaml:"autoPush,omitempty" json:"autoPush,omitempty"`
	PullInterval  string   `yaml:"pullInterval,omitempty" json:"pullInterval,omitempty"`
	RetryInterval string   `yaml:"retryInterval,omitempty" json:"retryInterval,omitempty"`
	Auth          *GitAuth `yaml:"auth,omitempty" json:"auth,omitempty"`
}

type GitAuth struct {
	Type          string `yaml:"type" json:"type"`
	SSHPrivateKey string `yaml:"sshPrivateKey,omitempty" json:"sshPrivateKey,omitempty"`
	Token         string `yaml:"token,omitempty" json:"token,omitempty"`
	Username      string `yaml:"username,omitempty" json:"username,omitempty"`
}

type Updates struct {
	Channel   string `yaml:"channel,omitempty" json:"channel,omitempty"`
	Automatic *bool  `yaml:"automatic,omitempty" json:"automatic,omitempty"`
	Window    string `yaml:"window,omitempty" json:"window,omitempty"`
}

type ThreatFeeds struct {
	Enabled         bool         `yaml:"enabled" json:"enabled"`
	RefreshInterval string       `yaml:"refreshInterval,omitempty" json:"refreshInterval,omitempty"`
	Sources         []ThreatFeed `yaml:"sources,omitempty" json:"sources,omitempty"`
}

type ThreatFeed struct {
	Name    string `yaml:"name" json:"name"`
	URL     string `yaml:"url" json:"url"`
	Enabled *bool  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// ---- effective values (schema defaults applied) ----------------------------------------

// HTTPSPort is the management port (DM-35), default 443.
func (s SystemSpec) HTTPSPort() int {
	if s.Admin != nil && s.Admin.HTTPSPort != 0 {
		return s.Admin.HTTPSPort
	}
	return 443
}

// SessionTimeout is the UI session lifetime (R-043.4, R-043.5), default 8h.
func (s SystemSpec) SessionTimeout() time.Duration {
	if s.Admin != nil {
		if d, ok := ParseDuration(s.Admin.SessionTimeout); ok && d > 0 {
			return d
		}
	}
	return 8 * time.Hour
}

// Lockout returns the brute-force threshold and duration (R-043.2),
// default 5 attempts and 10 minutes.
func (s SystemSpec) LockoutPolicy() (int, time.Duration) {
	n, d := 5, 10*time.Minute
	if s.Admin != nil && s.Admin.Lockout != nil {
		if s.Admin.Lockout.Threshold > 0 {
			n = s.Admin.Lockout.Threshold
		}
		if v, ok := ParseDuration(s.Admin.Lockout.Duration); ok && v > 0 {
			d = v
		}
	}
	return n, d
}

func (s SystemSpec) ThreatFeedsOn() bool {
	if s.ThreatFeeds == nil || !s.ThreatFeeds.Enabled {
		return false
	}
	for _, f := range s.ThreatFeeds.Sources {
		if f.Enabled == nil || *f.Enabled {
			return true
		}
	}
	return false
}
