// Package bizapi is a Go implementation of Google API for Business.
// This package include features like generating keys for clients,
// signing request URLs using the generated keys, and checking whether
// a signature in a request URL is a valid one of the URL so the
// service should proceed.  More technical information about Google
// API for Business can be found here:
// https://developers.google.com/maps/documentation/business/webservices/auth
package bizapi

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// EncodeUrlSafeBase64 encodes a URL using base64 and then with "+"
// replaced by "-" and with "/" replaced by "_".  Such an encoded URL
// can be used in a URL without the need for URL encoding.
func EncodeUrlSafeBase64(str []byte) string {
	encoded := base64.StdEncoding.EncodeToString(str)
	return strings.Replace(strings.Replace(encoded, "+", "-", -1), "/", "_", -1)
}

// DecodeUrlSafeBase64 decodes a URL that was URL-safe base64 encoded
// using EncodeUrlSafeBase64.
func DecodeUrlSafeBase64(wiered string) ([]byte, error) {
	urlUnsafeWiered := strings.Replace(strings.Replace(wiered, "-", "+", -1), "_", "/", -1)
	encoded, e := base64.StdEncoding.DecodeString(urlUnsafeWiered)
	if e != nil {
		return nil, fmt.Errorf("base64.StdEncoding.DecodeString failed: %v", e)
	}
	return encoded, nil
}

// GenerateKey invokes crypto.rsa.GenerateKey to randomly generate a
// key for signing request URLs.  The returned key was URL-safe base64
// encoded, and can be used by CreateSignature to sign a URL.
func GenerateKey() (string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 128)
	if err != nil {
		return "", fmt.Errorf("rsa.GeneratedKey failed: %v", err)
	}

	encodedPrivateKey := EncodeUrlSafeBase64(
		pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
			}))

	return encodedPrivateKey, nil
}

// CreateSignature computes a signature of the path and raw query part
// of an URL, where key should be a string generated by
// EncodeUrlSafeBase64.
func CreateSignature(requestUrl *url.URL, key string) (string, error) {
	values, err := url.ParseQuery(requestUrl.RawQuery)
	if err != nil {
		return "", fmt.Errorf("url.ParseQuery failed: %v", err)
	}
	if _, presents := values["client"]; !presents {
		return "", errors.New("A to-be-encoded URL must have the client query")
	}
	if _, presents := values["signature"]; presents {
		return "", errors.New("A to-be-encoded URL must NOT have the signature query")
	}

	// Important: url.Values.Encode() sorts values by their keys.
	urlToSign := requestUrl.Path + "?" + requestUrl.RawQuery

	decodedKey, err := DecodeUrlSafeBase64(key)
	if err != nil {
		return "", fmt.Errorf("DecodeUrlSafeBase64 failed: %v", err)
	}

	crypt := hmac.New(sha1.New, decodedKey)
	crypt.Write([]byte(urlToSign))
	signature := crypt.Sum(nil)
	return EncodeUrlSafeBase64(signature), nil
}

// CheckSignedUrl checks that a URL was attached with its signature
// and it ready to be sent to the server.
func CheckSignedUrl(rawUrl string) (*url.URL, url.Values, error) {
	parsedUrl, e := url.Parse(rawUrl)
	if e != nil {
		return nil, nil, fmt.Errorf("url.Parse(rawUrl=%s) failed: %v", rawUrl, e)
	}
	values, e := url.ParseQuery(parsedUrl.RawQuery)
	if e != nil {
		return nil, nil, fmt.Errorf("url.ParseQuery(parseUrl.RawQuery=%s) failed: %v", parsedUrl.RawQuery, e)
	}
	if v, present := values["client"]; !present || len(v) != 1 {
		return nil, nil, errors.New("Request URL must contain exactly one \"client\" parameter.")
	}
	if v, present := values["signature"]; !present || len(v) != 1 {
		return nil, nil, errors.New("Request URL must contain exactly one \"signature\" parameter.")
	}
	return parsedUrl, values, nil
}

// SignUrl computes the signature of a URL and attaches the signature
// with the URL as a parameter in the format of
// "&signature=<computed-signature".  It is important that the URL
// also contains a "client" parameter, which will be used by
// Authenticate to retrieved the key of the client.
func SignUrl(rawUrl, key string) (string, error) {
	url, e := url.Parse(rawUrl)
	if e != nil {
		return "", fmt.Errorf("url.Parse(rawUrl=%s) failed: %v", rawUrl, e)
	}
	signature, e := CreateSignature(url, key)
	if e != nil {
		return "", fmt.Errorf("CreateSignature(url=%s, key=%s) failed: %v", url, key, e)
	}
	return url.Scheme + "://" + url.Host + url.Path + "?" + url.RawQuery + "&signature=" + signature, nil
}

type KeyRepository map[string]string

// LoadKeyRepository loads client ID and key pairs.  Then people can
// call Authenticate to check whether a request URL is valid.
func LoadKeyRepository(reader io.Reader) (KeyRepository, error) {
	repo := make(map[string]string)
	s := bufio.NewScanner(reader)
	for s.Scan() {
		line := s.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		fields := strings.Split(line, " ")
		if len(fields) != 2 {
			return nil, fmt.Errorf("Every line must contains two fields separated by a space: %s", line)
		}
		repo[fields[0]] = fields[1]
	}
	return repo, nil
}

// Autheticate checks that the signature parameter in the request URL
// comfront the client parameter.  If it returns no error, the caller
// can find client ID in returned url.Values value.
func (c KeyRepository) Authenticate(rawUrl string) (*url.URL, url.Values, error) {
	parsedUrl, values, e := CheckSignedUrl(rawUrl)
	if e != nil {
		return nil, nil, fmt.Errorf("CheckSignedUrl failed: %v", e)
	}

	key, present := c[values["client"][0]]
	if !present {
		return nil, nil, fmt.Errorf("Unknown client: %s", values["client"])
	}

	attachedSignature := values["signature"][0]

	lastQuery := strings.LastIndex(rawUrl, "&signature=")
	if lastQuery == -1 {
		return nil, nil, fmt.Errorf("Cannot find &signatue= in rawUrl: %s", rawUrl)
	}

	urlWithoutSignature, e := url.Parse(rawUrl[0:lastQuery])
	if e != nil {
		return nil, nil, fmt.Errorf("url.Parse failed: %v", e)
	}
	signature, e := CreateSignature(urlWithoutSignature, key)
	if e != nil {
		return nil, nil, fmt.Errorf("CreateSignature failed: %v", e)
	}

	if attachedSignature != signature {
		return nil, nil, fmt.Errorf("Attached signature %s is not equal to computed signature %s",
			attachedSignature, signature)
	}
	return parsedUrl, values, nil
}
