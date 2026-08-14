package main

import (
	"fmt"
	"log"
	"os"
	"sort"
)

var logger = log.New(os.Stderr, "", log.LstdFlags)

var sortChoices = []string{"age", "rate", "country", "score", "delay"}

// infof logs to stderr when --verbose is set.
func infof(opts *Options, format string, args ...any) {
	if opts.Verbose {
		logger.Printf(format, args...)
	}
}

// run executes the amc pipeline and returns the exit code.
func run(argv []string) int {
	opts, err := parseFlags(argv)
	if err != nil {
		if hr, ok := err.(*helpRequested); ok {
			fmt.Fprint(os.Stdout, hr.usage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "amc: %v\n", err)
		return 2
	}

	configTokens, err := loadConfigTokens(opts.Config)
	if err != nil {
		if opts.Config != defaultConfigPath {
			fmt.Fprintf(os.Stderr, "amc: cannot read config %s: %v\n", opts.Config, err)
			return 1
		}
		configTokens = nil
	}

	merged := append(append([]string{}, configTokens...), argv...)

	opts, err = parseFlags(merged)
	if err != nil {
		if hr, ok := err.(*helpRequested); ok {
			fmt.Fprint(os.Stdout, hr.usage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "amc: %v\n", err)
		return 2
	}

	if opts.Sort != "" && !contains(sortChoices, opts.Sort) {
		fmt.Fprintf(os.Stderr, "amc: invalid sort criterion %q (choose from %v)\n", opts.Sort, sortChoices)
		return 2
	}

	status, retrieved, err := fetchStatus(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "amc: %v\n", err)
		return 1
	}

	if opts.ListCountries {
		fmt.Print(formatCountries(status.URLs))
		return 0
	}

	mirrors, err := applyFilters(status.URLs, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "amc: %v\n", err)
		return 2
	}

	// Apply limits and the final sort in the same order as the original.
	if opts.Latest > 0 {
		sortMirrors(mirrors, "age", nil)
		if len(mirrors) > opts.Latest {
			mirrors = mirrors[:opts.Latest]
		}
	}
	if opts.Score > 0 {
		sort.Slice(mirrors, func(i, j int) bool { return mirrors[i].Score > mirrors[j].Score })
		if len(mirrors) > opts.Score {
			mirrors = mirrors[:opts.Score]
		}
	}
	if opts.Fastest > 0 {
		rateMirrors(mirrors, opts)
		sortMirrors(mirrors, "rate", nil)
		if len(mirrors) > opts.Fastest {
			mirrors = mirrors[:opts.Fastest]
		}
	}
	if opts.Sort == "rate" && opts.Fastest == 0 {
		rateMirrors(mirrors, opts)
	}
	if opts.Sort != "" && !(opts.Sort == "rate" && opts.Fastest > 0) {
		sortMirrors(mirrors, opts.Sort, opts.Countries)
	}
	if opts.Number > 0 && len(mirrors) > opts.Number {
		mirrors = mirrors[:opts.Number]
	}

	if opts.Info {
		if len(mirrors) == 0 {
			fmt.Fprintln(os.Stderr, "amc: no mirrors found")
			return 1
		}
		fmt.Print(formatMirrorInfo(mirrors))
		return 0
	}

	if len(mirrors) == 0 {
		fmt.Fprintln(os.Stderr, "amc: no mirrors found")
		return 1
	}

	output := formatMirrorlist(status, mirrors, retrieved, opts, argv)
	if opts.Save != "" {
		if err := os.WriteFile(opts.Save, []byte(output), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "amc: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Print(output)
	return 0
}

func main() {
    os.Exit(run(os.Args[1:]))
}
