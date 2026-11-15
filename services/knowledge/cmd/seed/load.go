package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"ego/platform/logger"
)

// readSeedFile đọc file JSON vào v. Trả false (không lỗi) khi file không tồn tại để phần
// seed đó được bỏ qua.
func readSeedFile(path string, v any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		logger.Log.Warn().Str("path", path).Msg("[SEED] File not found, skipping")
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	return true, nil
}

// optionalString trả nil cho chuỗi rỗng (sau khi trim), còn lại trả con trỏ tới bản đã trim.
func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// nonEmpty bỏ các phần tử rỗng và trim phần còn lại; luôn trả slice khác nil để ghi jsonb '[]'.
func nonEmpty(items []string) []string {
	res := make([]string, 0, len(items))
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			res = append(res, s)
		}
	}
	return res
}
