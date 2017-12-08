package config

import "io"
import "bufio"
import "fmt"

const limit = 2000

func Parse(r io.Reader) ([]string, error) {
	s := bufio.NewScanner(r)

	res := make([]string, 0, limit)
	for i := 0; s.Scan(); i++ {
		if i == limit {
			return nil, fmt.Errorf("Config exceeds %d lines size limit", limit)
		}
		res = append(res, s.Text())
	}

	if s.Err() != nil {
		return nil, s.Err()
	}

	return res, nil
}
