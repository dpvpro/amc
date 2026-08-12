package main

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// applyFilters keeps only the mirrors that satisfy every active filter, in
// the same order as the Python version.
func applyFilters(mirrors []Mirror, opts *Options) ([]Mirror, error) {
	includes, err := compileAll(opts.Include)
	if err != nil {
		return nil, fmt.Errorf("invalid --include regex: %w", err)
	}
	excludes, err := compileAll(opts.Exclude)
	if err != nil {
		return nil, fmt.Errorf("invalid --exclude regex: %w", err)
	}

	countries := make([]string, 0, len(opts.Countries))
	hasCountries := len(opts.Countries) > 0
	allowAny := false
	for _, c := range opts.Countries {
		u := strings.ToUpper(c)
		if u == "*" {
			allowAny = true
		}
		countries = append(countries, u)
	}
	hasProtocols := len(opts.Protocols) > 0
	minCompletion := opts.CompletionPercent / 100

	var out []Mirror
	for _, m := range mirrors {
		if !keep(m, minCompletion, countries, hasCountries, allowAny,
			hasProtocols, opts.Protocols, includes, excludes, opts.Age,
			opts.Delay, opts.HasDelay, opts.Isos, opts.IPv4, opts.IPv6) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func keep(
	m Mirror,
	minCompletion float64,
	countries []string,
	hasCountries, allowAny, hasProtocols bool,
	protocols []string,
	includes, excludes []*regexp.Regexp,
	age float64,
	delay float64, hasDelay bool,
	isos, ipv4, ipv6 bool,
) bool {
	// Skip mirrors that never synchronized.
	if m.LastSync == nil {
		return false
	}
	t, err := parseLastSync(*m.LastSync)
	if err != nil {
		return false
	}
	m.lastSyncTime = t

	if m.CompletionPct < minCompletion {
		return false
	}
	if hasCountries && !allowAny {
		country := strings.ToUpper(m.Country)
		code := strings.ToUpper(m.CountryCode)
		if !contains(countries, country) && !contains(countries, code) {
			return false
		}
	}
	if hasProtocols && !contains(protocols, m.Protocol) {
		return false
	}
	if len(includes) > 0 && !anyMatch(includes, m.URL) {
		return false
	}
	if len(excludes) > 0 && anyMatch(excludes, m.URL) {
		return false
	}
	if age > 0 && m.lastSyncTime.Add(time.Duration(age*3600)*time.Second).Before(time.Now()) {
		return false
	}
	if hasDelay && m.Delay > delay*3600 {
		return false
	}
	if isos && !m.Isos {
		return false
	}
	if ipv4 && !m.IPv4 {
		return false
	}
	if ipv6 && !m.IPv6 {
		return false
	}
	return true
}

// compileAll compiles the given patterns, stopping at the first error.
func compileAll(patterns []string) ([]*regexp.Regexp, error) {
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		res = append(res, re)
	}
	return res, nil
}

func anyMatch(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}

// sortMirrors sorts mirrors by criterion in place.
func sortMirrors(mirrors []Mirror, criterion string, countries []string) {
	switch criterion {
	case "age":
		sort.Slice(mirrors, func(i, j int) bool {
			return mirrors[i].lastSyncTime.After(mirrors[j].lastSyncTime)
		})
	case "rate":
		sort.Slice(mirrors, func(i, j int) bool {
			return mirrors[i].rate > mirrors[j].rate
		})
	case "country":
		if len(countries) > 0 {
			sort.Slice(mirrors, countrySortKey(mirrors, countries))
		} else {
			sort.Slice(mirrors, func(i, j int) bool {
				return mirrors[i].Country < mirrors[j].Country
			})
		}
	case "score":
		sort.Slice(mirrors, func(i, j int) bool {
			return mirrors[i].Score < mirrors[j].Score
		})
	case "delay":
		sort.Slice(mirrors, func(i, j int) bool {
			return mirrors[i].Delay < mirrors[j].Delay
		})
	}
}

// countrySortKey ranks mirrors by the priority order of the given countries.
// An asterisk sets the default rank for countries not listed explicitly.
func countrySortKey(mirrors []Mirror, priorities []string) func(i, j int) bool {
	up := make([]string, len(priorities))
	for i, p := range priorities {
		up[i] = strings.ToUpper(p)
	}
	defaultRank := len(up)
	for i, p := range up {
		if p == "*" {
			defaultRank = i
			break
		}
	}
	rank := func(m Mirror) (int, string) {
		country := strings.ToUpper(m.Country)
		code := strings.ToUpper(m.CountryCode)
		for i, p := range up {
			if p == country {
				return i, country
			}
		}
		for i, p := range up {
			if p == code {
				return i, country
			}
		}
		return defaultRank, country
	}
	return func(i, j int) bool {
		ri, ci := rank(mirrors[i])
		rj, cj := rank(mirrors[j])
		if ri != rj {
			return ri < rj
		}
		return ci < cj
	}
}
