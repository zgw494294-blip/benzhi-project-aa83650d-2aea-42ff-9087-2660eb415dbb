package main

import "testing"

func TestValidateAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:19081", "localhost:19082", "[::1]:19083"} {
		if err := validateAddress(address); err != nil {
			t.Errorf("回环地址 %s 被拒绝：%v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:19081", "127.0.0.1:0", "example.com:19081", "bad"} {
		if err := validateAddress(address); err == nil {
			t.Errorf("不安全地址 %s 未被拒绝", address)
		}
	}
}
