//
// Copyright 2022 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/franchb/cosign/v2/pkg/cosign/attestation"
	"github.com/franchb/cosign/v2/pkg/cosign/bundle"
	"github.com/franchb/cosign/v2/pkg/oci"
	"github.com/franchb/cosign/v2/pkg/oci/static"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/in-toto/in-toto-golang/in_toto"
)

type failingAttestation struct {
}

func (fa *failingAttestation) Payload() ([]byte, error) {
	return nil, fmt.Errorf("inducing test failure")
}
func (fa *failingAttestation) Annotations() (map[string]string, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Signature() ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Base64Signature() (string, error) {
	return "", fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Cert() (*x509.Certificate, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Chain() ([]*x509.Certificate, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Bundle() (*bundle.RekorBundle, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) RFC3161Timestamp() (*bundle.RFC3161Timestamp, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Digest() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) DiffID() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Compressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Uncompressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) Size() (int64, error) {
	return 0, fmt.Errorf("unimplemented")
}
func (fa *failingAttestation) MediaType() (types.MediaType, error) {
	return types.DockerConfigJSON, fmt.Errorf("unimplemented")
}

var _ oci.Signature = (*failingAttestation)(nil)

const (
	// Result of "echo 'nottotostatement' | base64"
	//	invalidTotoStatement = "bm90dG90b3N0YXRlbWVudAo="
	invalidTotoStatement = `{"payloadType":"application/vnd.in-toto+json","payload":"bm90dG90b3N0YXRlbWVudAo"}`
)

func checkFailure(t *testing.T, want string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("Expected error, got none")
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Failed to get the expected error of %q, got: %s", want, err)
	}
}

func TestFailures(t *testing.T) {
	tests := []struct {
		payload          string
		predicateType    string
		wantErrSubstring string
	}{{payload: "", wantErrSubstring: "unmarshaling payload data"}, {payload: "{badness", wantErrSubstring: "unmarshaling payload data"},
		{payload: `{"payloadType":"notmarshallable}`, wantErrSubstring: "unmarshaling payload data"},
		{payload: `{"payload":"shou!ln'twork"}`, wantErrSubstring: "decoding payload"},
		{payload: `{"payloadType":"finebutnopayload"}`, wantErrSubstring: "could not find payload"},
		{payload: invalidTotoStatement, wantErrSubstring: "decoding payload: illegal base64"},
	}
	for _, tc := range tests {
		att, err := static.NewSignature([]byte(tc.payload), "")
		if err != nil {
			t.Fatal("Failed to create static.NewSignature: ", err)
		}
		predicateType := tc.predicateType
		if predicateType == "" {
			predicateType = "custom"
		}
		_, _, err = AttestationToPayloadJSON(context.TODO(), predicateType, att)
		checkFailure(t, tc.wantErrSubstring, err)
	}
}

// TestMalformedPayload tests various non-predicate specific failures that
// are done even before we start processing the payload.
// This just stands alone since didn't want to complicate above tests with
// constructing different attestations there.
func TestErroringPayload(t *testing.T) {
	// Payload() call fails
	_, _, err := AttestationToPayloadJSON(context.TODO(), "custom", &failingAttestation{})
	checkFailure(t, "inducing test failure", err)
}
func TestAttestationToPayloadJson(t *testing.T) {
	dir := "valid"
	files := getDirFiles(t, dir)
	for _, fileName := range files {
		bytes := readAttestationFromTestFile(t, dir, fileName)
		ociSig, err := static.NewSignature(bytes, "")
		if err != nil {
			t.Fatal("Failed to create static.NewSignature: ", err)
		}
		jsonBytes, gotPredicateType, err := AttestationToPayloadJSON(context.TODO(), fileName, ociSig)
		if err != nil {
			t.Fatalf("Failed to convert : %s", err)
		}
		switch fileName {
		case "custom":
			var intoto in_toto.Statement
			if err := json.Unmarshal(jsonBytes, &intoto); err != nil {
				t.Fatalf("[%s] Wanted custom statement, can't unmarshal to it: %v", fileName, err)
			}
			checkPredicateType(t, attestation.CosignCustomProvenanceV01, intoto.PredicateType)
			checkPredicateType(t, gotPredicateType, intoto.PredicateType)
		case "vuln":
			var vulnStatement attestation.CosignVulnStatement
			if err := json.Unmarshal(jsonBytes, &vulnStatement); err != nil {
				t.Fatalf("[%s] Wanted vuln statement, can't unmarshal to it: %v", fileName, err)
			}
			checkPredicateType(t, attestation.CosignVulnProvenanceV01, vulnStatement.PredicateType)
			checkPredicateType(t, gotPredicateType, vulnStatement.PredicateType)
		case "default":
			t.Fatal("non supported predicate file")
		}
	}
}

type myPayloadProvider struct {
	payload []byte
}

func (m *myPayloadProvider) Payload() ([]byte, error) {
	return m.payload, nil
}

// assert that myPayloadProvider implements PayloadProvider
var _ PayloadProvider = &myPayloadProvider{}

// TestPayloadProvider tests that the PayloadProvider interface is working as expected.
func TestPayloadProvider(t *testing.T) {
	// Control: oci.Signature
	attestationBytes := readAttestationFromTestFile(t, "valid", "vuln")
	ociSig, err := static.NewSignature(attestationBytes, "")
	if err != nil {
		t.Fatal("Failed to create static.NewSignature: ", err)
	}
	jsonBytes, gotPredicateType, err := AttestationToPayloadJSON(context.TODO(), "vuln", ociSig)
	if err != nil {
		t.Fatalf("Failed to convert : %s", err)
	}
	if len(jsonBytes) == 0 {
		t.Fatalf("Failed to get jsonBytes")
	}
	if gotPredicateType != attestation.CosignVulnProvenanceV01 {
		t.Fatalf("Did not get expected predicateType, want: %s got: %s", attestation.CosignVulnProvenanceV01, gotPredicateType)
	}

	// Test: myPayloadProvider
	provider := &myPayloadProvider{payload: attestationBytes}
	jsonBytes2, gotPredicateType2, err := AttestationToPayloadJSON(context.TODO(), "vuln", provider)
	if err != nil {
		t.Fatalf("Failed to convert : %s", err)
	}
	if !bytes.Equal(jsonBytes, jsonBytes2) {
		t.Fatalf("Expected same jsonBytes, got different")
	}
	if gotPredicateType != gotPredicateType2 {
		t.Fatalf("Expected same predicateType, got different")
	}
}

func checkPredicateType(t *testing.T, want, got string) {
	t.Helper()
	if want != got {
		t.Errorf("Did not get expected predicateType, want: %s got: %s", want, got)
	}
}

func readAttestationFromTestFile(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fmt.Sprintf("testdata/%s/%s", dir, name))
	if err != nil {
		t.Fatalf("Failed to read file : %s ReadFile() = %s", name, err)
	}
	return b
}

func getDirFiles(t *testing.T, dir string) []string {
	files, err := os.ReadDir(fmt.Sprintf("testdata/%s", dir))
	if err != nil {
		t.Fatalf("Failed to read dir : %s ReadFile() = %s", dir, err)
	}
	ret := []string{}
	for _, file := range files {
		ret = append(ret, file.Name())
	}
	return ret
}
