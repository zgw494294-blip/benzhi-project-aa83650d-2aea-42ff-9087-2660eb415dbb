package main

import "testing"

func TestConfigDefaultsToHighLoopback(t *testing.T) {
	t.Setenv("PORT", "")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:19081" {
		t.Fatalf("默认地址=%s", cfg.Addr)
	}
}

func TestConfigRejectsWildcard(t *testing.T) {
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}); err == nil {
		t.Fatal("未拒绝非回环地址")
	}
}
