package main

import "testing"

func TestEmailP1B2DoNotCreateOwnCRMContact(t *testing.T) {
	tests := []struct {
		sender, account string
		want            bool
	}{
		{"gerenciaworktic@gmail.com", "gerenciaworktic@gmail.com", false},
		{" GerenciaWorktic@GMAIL.COM ", "gerenciaworktic@gmail.com", false},
		{"invictuxtrade@gmail.com", "gerenciaworktic@gmail.com", true},
		{"", "gerenciaworktic@gmail.com", false},
	}
	for _, tc := range tests {
		if got := emailP1B2ShouldCreateContact(tc.sender, tc.account); got != tc.want {
			t.Errorf("unexpected CRM contact decision: got %v want %v", got, tc.want)
		}
	}
}
