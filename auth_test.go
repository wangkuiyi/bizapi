package bizapi

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

var (
	kUrl           = "http://maps.googleapis.com/maps/api/geocode/json?address=New+York&sensor=false&client=clientID"
	kPrivateKey    = "vNIXE0xscrmjlyV-12Nj_BvUPaw="
	kSignature     = "KrU1TzVQM7Ur0i8i7K3huiw3MsA="
	kFullSignedUrl = "http://maps.googleapis.com/maps/api/geocode/json?address=New+York&sensor=false&client=clientID&signature=KrU1TzVQM7Ur0i8i7K3huiw3MsA="
)

func TestCreateSignature(t *testing.T) {
	u, _ := url.Parse(kUrl)
	signature, err := CreateSignature(u, kPrivateKey)
	if err != nil {
		t.Errorf("CreateSignature failed: %v", err)
	}
	if signature != kSignature {
		t.Errorf("Expecting signature: %s, got %s", kSignature, signature)
	}
}

func TestSignUrl(t *testing.T) {
	signedUrl, err := SignUrl(kUrl, kPrivateKey)
	if err != nil {
		t.Errorf("CreateSignature failed: %v", err)
	}
	if signedUrl != kFullSignedUrl {
		t.Errorf("Expecting full signed URL:\n%s\ngot\n%s", kFullSignedUrl, signedUrl)
	}
}

// TODO(wyi): better test or just remove it.
func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Errorf("GenerateKey failed: %v", err)
	}
	fmt.Println(len(key))
}

func TestLoadKeyRepository(t *testing.T) {
	r := strings.NewReader("clientID vNIXE0xscrmjlyV-12Nj_BvUPaw=\nyiw something")
	repo, e := LoadKeyRepository(r)
	if e != nil {
		t.Errorf("LoadKeyRepository failed.")
	}
	if len(repo) != 2 || repo["clientID"] != "vNIXE0xscrmjlyV-12Nj_BvUPaw=" || repo["yiw"] != "something" {
		t.Errorf("Loaded wrong contents")
	}
}

func TestCheckSignedUrl(t *testing.T) {
	_, _, e := CheckSignedUrl("http://company.com?signature=xxx")
	if e == nil {
		t.Errorf("Expecting failure due to no client query")
	}

	_, _, e = CheckSignedUrl("http://company.com?client=xxx")
	if e == nil {
		t.Errorf("Expecting failure due to no signature query")
	}
}

func TestAuthenticate(t *testing.T) {
	signedUrl, err := SignUrl(kUrl, kPrivateKey)
	if err != nil {
		t.Errorf("CreateSignature failed: %v", err)
	}

	r := KeyRepository{"clientID": kPrivateKey}
	_, values, e := r.Authenticate(signedUrl)
	if e != nil {
		t.Errorf("Authenticate got unexpected error: %v", e)
	}
	if values["client"] == nil || values["client"][0] != "clientID" {
		t.Errorf("No client ID found in the returned url.Values variable.")
	}

	r = KeyRepository{"clientID": "invalidBased64Key"}
	if _, _, e := r.Authenticate(signedUrl); e == nil {
		t.Errorf("Invalid based64-encoded key passed the check of Autheticate!: %v", e)
	}
}
