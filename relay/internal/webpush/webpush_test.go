package webpush

import (
	"testing"
)

func TestRoundTrip(t *testing.T) {
	app, err := NewSubscriber()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte(`{"title":"Neue Folge","body":"…"}`)
	body, err := Encrypt(app.Key.PublicKey(), app.Auth, msg, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 2048 {
		t.Fatalf("body is %d bytes, want one record of 2048", len(body))
	}
	got, err := app.Decrypt(body)
	if err != nil || string(got) != string(msg) {
		t.Fatalf("Decrypt = %q, %v", got, err)
	}
	other, _ := NewSubscriber()
	if _, err := other.Decrypt(body); err == nil {
		t.Fatal("another subscriber decrypted the message")
	}
	body[len(body)-1] ^= 1
	if _, err := app.Decrypt(body); err == nil {
		t.Fatal("a tampered message decrypted")
	}
	if _, err := Encrypt(app.Key.PublicKey(), app.Auth, make([]byte, 200), 200); err == nil {
		t.Fatal("plaintext larger than the record was accepted")
	}
}
