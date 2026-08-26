package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// Default values.
const (
	defaultURL               = "https://archlinux.org/mirrors/status/json/"
	defaultConnectionTimeout = 8
	defaultDownloadTimeout   = 8
	defaultCacheTimeout      = 300
)

// Options holds all parsed command-line options.
type Options struct {
	ConnectionTimeout int
	DownloadTimeout   int
	CacheTimeout      int
	URL               string
	Save              string
	Config            string
	Sort              string
	Threads           int
	Verbose           bool
	Info              bool
	ListCountries     bool

	Age               float64
	Delay             float64
	HasDelay          bool
	Countries         []string
	Fastest           int
	Include           []string
	Exclude           []string
	Latest            int
	Score             int
	Number            int
	Protocols         []string
	CompletionPercent float64
	Isos              bool
	IPv4              bool
	IPv6              bool
}

// defaultOptions returns the options with their default values.
func defaultOptions() *Options {
	return &Options{
		ConnectionTimeout: defaultConnectionTimeout,
		DownloadTimeout:   defaultDownloadTimeout,
		CacheTimeout:      defaultCacheTimeout,
		URL:               defaultURL,
		CompletionPercent: 100,
	}
}

// newFlagSet builds the flag set bound to opts.
func newFlagSet(opts *Options) *pflag.FlagSet {
	fs := pflag.NewFlagSet("amc", pflag.ContinueOnError)
	// pflag prints errors and usage itself; suppress that so this package
	// controls the exact output and exit codes.
	fs.SetOutput(io.Discard)

	fs.IntVar(&opts.ConnectionTimeout, "connection-timeout", defaultConnectionTimeout, "seconds to wait before a connection times out")
	fs.IntVar(&opts.DownloadTimeout, "download-timeout", defaultDownloadTimeout, "seconds to wait before a download times out")
	fs.IntVar(&opts.CacheTimeout, "cache-timeout", defaultCacheTimeout, "seconds the mirror status data may be cached")
	fs.StringVar(&opts.URL, "url", defaultURL, "URL of the mirror status JSON")
	fs.StringVar(&opts.Save, "save", "", "write the mirrorlist to this path instead of stdout")
	fs.StringVar(&opts.Sort, "sort", "", "sort by age, rate, country, score or delay")
	fs.IntVar(&opts.Threads, "threads", 8, "number of parallel rating downloads (1 = sequential)")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print extra information to stderr")
	fs.BoolVar(&opts.Info, "info", false, "print mirror information instead of a mirror list")
	fs.BoolVar(&opts.ListCountries, "list-countries", false, "list countries with a mirror count and exit")

	fs.StringVar(&opts.Config, "config", "", "read options from this config file")

	fs.Float64Var(&opts.Age, "age", 0, "only mirrors synchronized in the last n hours")
	fs.Float64Var(&opts.Delay, "delay", 0, "only mirrors with a reported sync delay of n hours or less")
	fs.StringSliceVar(&opts.Countries, "country", nil, "restrict to these countries (name or code, comma-separated, repeatable)")
	fs.IntVar(&opts.Fastest, "fastest", 0, "return the n fastest mirrors")
	fs.StringSliceVar(&opts.Include, "include", nil, "include servers matching this regex (repeatable)")
	fs.StringSliceVar(&opts.Exclude, "exclude", nil, "exclude servers matching this regex (repeatable)")
	fs.IntVar(&opts.Latest, "latest", 0, "limit to the n most recently synchronized mirrors")
	fs.IntVar(&opts.Score, "score", 0, "limit to the n mirrors with the highest score")
	fs.IntVar(&opts.Number, "number", 0, "return at most n mirrors")
	fs.StringSliceVar(&opts.Protocols, "protocol", nil, "match these protocols, comma-separated, repeatable")
	fs.Float64Var(&opts.CompletionPercent, "completion-percent", 100, "minimum completion percent [0-100]")
	fs.BoolVar(&opts.Isos, "isos", false, "only mirrors that host ISOs")
	fs.BoolVar(&opts.IPv4, "ipv4", false, "only mirrors that support IPv4")
	fs.BoolVar(&opts.IPv6, "ipv6", false, "only mirrors that support IPv6")

	return fs
}

// parseFlags builds the flag set and parses argv, returning the options.
func parseFlags(argv []string) (*Options, error) {
	opts := defaultOptions()
	fs := newFlagSet(opts)
	if err := fs.Parse(argv); err != nil {
		if err == pflag.ErrHelp {
			return nil, pflag.ErrHelp
		}
		return nil, fmt.Errorf("%v\n%s", err, usageText())
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument: %s", fs.Arg(0))
	}
	if opts.Threads < 1 {
		return nil, fmt.Errorf("invalid --threads value %d: must be at least 1", opts.Threads)
	}
	return opts, nil
}

// usageText renders the amc help text.
func usageText() string {
	return "Usage of amc:\n" + flagDefaults(newFlagSet(defaultOptions()))
}

// flagDefaults renders the flag help text.
func flagDefaults(fs *pflag.FlagSet) string {
	var b strings.Builder
	fs.SetOutput(&b)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
	return b.String()
}

// loadConfigTokens reads a config file: one flag per line, "#" comments and
// quoted values are supported.
func loadConfigTokens(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ' '
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var tokens []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) == 0 || strings.HasPrefix(record[0], "#") {
			continue
		}
		for _, field := range record {
			if field != "" {
				tokens = append(tokens, field)
			}
		}
	}
	return tokens, nil
}
