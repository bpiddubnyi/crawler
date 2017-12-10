package config

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

const limit = 2000

var (
	httpPrefix  = []byte("http://")
	httpsPrefix = []byte("https://")
)

func Parse(r io.Reader) ([]string, error) {
	s := bufio.NewScanner(r)

	res := make([]string, 0, limit)
	for i := 0; s.Scan(); i++ {
		if i == limit {
			return nil, fmt.Errorf("Config exceeds %d lines size limit", limit)
		}

		url := bytes.TrimSpace(s.Bytes())
		if !bytes.HasPrefix(url, httpPrefix) && !bytes.HasPrefix(url, httpsPrefix) {
			url = append([]byte("http://"), url...)
		}
		res = append(res, string(url))
	}

	if s.Err() != nil {
		return nil, s.Err()
	}

	return res, nil
}
