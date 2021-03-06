package srtp

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

const cipherContextAlgo = ProtectionProfileAes128CmHmacSha1_80
const defaultSsrc = 0

type rtpTestCase struct {
	sequenceNumber uint16
	encrypted      []byte
}

func TestKeyLen(t *testing.T) {
	if _, err := CreateContext([]byte{}, make([]byte, saltLen), cipherContextAlgo); err == nil {
		t.Errorf("CreateContext accepted a 0 length key")
	}

	if _, err := CreateContext(make([]byte, keyLen), []byte{}, cipherContextAlgo); err == nil {
		t.Errorf("CreateContext accepted a 0 length salt")
	}

	if _, err := CreateContext(make([]byte, keyLen), make([]byte, saltLen), cipherContextAlgo); err != nil {
		t.Errorf("CreateContext failed with a valid length key and salt: %v", err)
	}
}

func TestValidSessionKeys(t *testing.T) {
	masterKey := []byte{0xE1, 0xF9, 0x7A, 0x0D, 0x3E, 0x01, 0x8B, 0xE0, 0xD6, 0x4F, 0xA3, 0x2C, 0x06, 0xDE, 0x41, 0x39}
	masterSalt := []byte{0x0E, 0xC6, 0x75, 0xAD, 0x49, 0x8A, 0xFE, 0xEB, 0xB6, 0x96, 0x0B, 0x3A, 0xAB, 0xE6}

	expectedSessionKey := []byte{0xC6, 0x1E, 0x7A, 0x93, 0x74, 0x4F, 0x39, 0xEE, 0x10, 0x73, 0x4A, 0xFE, 0x3F, 0xF7, 0xA0, 0x87}
	expectedSessionSalt := []byte{0x30, 0xCB, 0xBC, 0x08, 0x86, 0x3D, 0x8C, 0x85, 0xD4, 0x9D, 0xB3, 0x4A, 0x9A, 0xE1}
	expectedSessionAuthTag := []byte{0xCE, 0xBE, 0x32, 0x1F, 0x6F, 0xF7, 0x71, 0x6B, 0x6F, 0xD4, 0xAB, 0x49, 0xAF, 0x25, 0x6A, 0x15, 0x6D, 0x38, 0xBA, 0xA4}

	c, err := CreateContext(masterKey, masterSalt, cipherContextAlgo)
	if err != nil {
		t.Errorf("CreateContext failed: %v", err)
	}

	sessionKey, err := c.generateSessionKey(labelSRTPEncryption)
	if err != nil {
		t.Errorf("generateSessionKey failed: %v", err)
	} else if !bytes.Equal(sessionKey, expectedSessionKey) {
		t.Errorf("Session Key % 02x does not match expected % 02x", sessionKey, expectedSessionKey)
	}

	sessionSalt, err := c.generateSessionSalt(labelSRTPSalt)
	if err != nil {
		t.Errorf("generateSessionSalt failed: %v", err)
	} else if !bytes.Equal(sessionSalt, expectedSessionSalt) {
		t.Errorf("Session Salt % 02x does not match expected % 02x", sessionSalt, expectedSessionSalt)
	}

	sessionAuthTag, err := c.generateSessionAuthTag(labelSRTPAuthenticationTag)
	if err != nil {
		t.Errorf("generateSessionAuthTag failed: %v", err)
	} else if !bytes.Equal(sessionAuthTag, expectedSessionAuthTag) {
		t.Errorf("Session Auth Tag % 02x does not match expected % 02x", sessionAuthTag, expectedSessionAuthTag)
	}
}

func TestValidPacketCounter(t *testing.T) {
	masterKey := []byte{0x0d, 0xcd, 0x21, 0x3e, 0x4c, 0xbc, 0xf2, 0x8f, 0x01, 0x7f, 0x69, 0x94, 0x40, 0x1e, 0x28, 0x89}
	masterSalt := []byte{0x62, 0x77, 0x60, 0x38, 0xc0, 0x6d, 0xc9, 0x41, 0x9f, 0x6d, 0xd9, 0x43, 0x3e, 0x7c}

	c, err := CreateContext(masterKey, masterSalt, cipherContextAlgo)
	if err != nil {
		t.Errorf("CreateContext failed: %v", err)
	}

	s := &ssrcState{ssrc: 4160032510}
	expectedCounter := []byte{0xcf, 0x90, 0x1e, 0xa5, 0xda, 0xd3, 0x2c, 0x15, 0x00, 0xa2, 0x24, 0xae, 0xae, 0xaf, 0x00, 0x00}
	counter := c.generateCounter(32846, s.rolloverCounter, s.ssrc, c.srtpSessionSalt)
	if !bytes.Equal(counter, expectedCounter) {
		t.Errorf("Session Key % 02x does not match expected % 02x", counter, expectedCounter)
	}
}

func TestRolloverCount(t *testing.T) {
	masterKey := []byte{0x0d, 0xcd, 0x21, 0x3e, 0x4c, 0xbc, 0xf2, 0x8f, 0x01, 0x7f, 0x69, 0x94, 0x40, 0x1e, 0x28, 0x89}
	masterSalt := []byte{0x62, 0x77, 0x60, 0x38, 0xc0, 0x6d, 0xc9, 0x41, 0x9f, 0x6d, 0xd9, 0x43, 0x3e, 0x7c}

	c, err := CreateContext(masterKey, masterSalt, cipherContextAlgo)
	if err != nil {
		t.Errorf("CreateContext failed: %v", err)
	}

	s := &ssrcState{ssrc: defaultSsrc}

	// Set initial seqnum
	c.updateRolloverCount(65530, s)

	// We rolled over to 0
	c.updateRolloverCount(0, s)
	if s.rolloverCounter != 1 {
		t.Errorf("rolloverCounter was not updated after it crossed 0")
	}

	c.updateRolloverCount(65530, s)
	if s.rolloverCounter != 0 {
		t.Errorf("rolloverCounter was not updated when it rolled back, failed to handle out of order")
	}

	c.updateRolloverCount(5, s)
	if s.rolloverCounter != 1 {
		t.Errorf("rolloverCounter was not updated when it rolled over initial, to handle out of order")
	}

	c.updateRolloverCount(6, s)
	c.updateRolloverCount(7, s)
	c.updateRolloverCount(8, s)
	if s.rolloverCounter != 1 {
		t.Errorf("rolloverCounter was improperly updated for non-significant packets")
	}
}

func buildTestContext() (*Context, error) {
	masterKey := []byte{0x0d, 0xcd, 0x21, 0x3e, 0x4c, 0xbc, 0xf2, 0x8f, 0x01, 0x7f, 0x69, 0x94, 0x40, 0x1e, 0x28, 0x89}
	masterSalt := []byte{0x62, 0x77, 0x60, 0x38, 0xc0, 0x6d, 0xc9, 0x41, 0x9f, 0x6d, 0xd9, 0x43, 0x3e, 0x7c}

	return CreateContext(masterKey, masterSalt, cipherContextAlgo)
}

func TestRTPInvalidAuth(t *testing.T) {
	masterKey := []byte{0x0d, 0xcd, 0x21, 0x3e, 0x4c, 0xbc, 0xf2, 0x8f, 0x01, 0x7f, 0x69, 0x94, 0x40, 0x1e, 0x28, 0x89}
	invalidSalt := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	encryptContext, err := buildTestContext()
	if err != nil {
		t.Fatal(err)
	}

	invalidContext, err := CreateContext(masterKey, invalidSalt, cipherContextAlgo)
	if err != nil {
		t.Errorf("CreateContext failed: %v", err)
	}

	for _, testCase := range rtpTestCases {
		pkt := &rtp.Packet{Payload: rtpTestCaseDecrypted, Header: rtp.Header{SequenceNumber: testCase.sequenceNumber}}
		pktRaw, err := pkt.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := encryptContext.EncryptRTP(nil, pktRaw, nil)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := invalidContext.DecryptRTP(nil, out, nil); err == nil {
			t.Errorf("Managed to decrypt with incorrect salt for packet with SeqNum: %d", testCase.sequenceNumber)
		}
	}
}

var rtpTestCaseDecrypted = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
var rtpTestCases = []rtpTestCase{
	{
		sequenceNumber: 5000,
		encrypted:      []byte{0x6d, 0xd3, 0x7e, 0xd5, 0x99, 0xb7, 0x2d, 0x28, 0xb1, 0xf3, 0xa1, 0xf0, 0xc, 0xfb, 0xfd, 0x8},
	},
	{
		sequenceNumber: 5001,
		encrypted:      []byte{0xda, 0x47, 0xb, 0x2a, 0x74, 0x53, 0x65, 0xbd, 0x2f, 0xeb, 0xdc, 0x4b, 0x6d, 0x23, 0xf3, 0xde},
	},
	{
		sequenceNumber: 5002,
		encrypted:      []byte{0x6e, 0xa7, 0x69, 0x8d, 0x24, 0x6d, 0xdc, 0xbf, 0xec, 0x2, 0x1c, 0xd1, 0x60, 0x76, 0xc1, 0xe},
	},
	{
		sequenceNumber: 5003,
		encrypted:      []byte{0x24, 0x7e, 0x96, 0xc8, 0x7d, 0x33, 0xa2, 0x92, 0x8d, 0x13, 0x8d, 0xe0, 0x76, 0x9f, 0x8, 0xdc},
	},
	{
		sequenceNumber: 5004,
		encrypted:      []byte{0x75, 0x43, 0x28, 0xe4, 0x3a, 0x77, 0x59, 0x9b, 0x2e, 0xdf, 0x7b, 0x12, 0x68, 0xb, 0x57, 0x49},
	},
}

func TestRTPLifecyleNewAlloc(t *testing.T) {
	assert := assert.New(t)

	encryptContext, err := buildTestContext()
	if err != nil {
		t.Fatal(err)
	}

	decryptContext, err := buildTestContext()
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range rtpTestCases {
		decryptedPkt := &rtp.Packet{Payload: rtpTestCaseDecrypted, Header: rtp.Header{SequenceNumber: testCase.sequenceNumber}}
		decryptedRaw, err := decryptedPkt.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		encryptedPkt := &rtp.Packet{Payload: testCase.encrypted, Header: rtp.Header{SequenceNumber: testCase.sequenceNumber}}
		encryptedRaw, err := encryptedPkt.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		actualEncrypted, err := encryptContext.EncryptRTP(nil, decryptedRaw, nil)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equalf(actualEncrypted, encryptedRaw, "RTP packet with SeqNum invalid encryption: %d", testCase.sequenceNumber)

		actualDecrypted, err := decryptContext.DecryptRTP(nil, encryptedRaw, nil)
		if err != nil {
			t.Fatal(err)
		} else if bytes.Equal(encryptedRaw[:len(encryptedRaw)-authTagSize], actualDecrypted) {
			t.Fatal("DecryptRTP improperly encrypted in place")
		}

		assert.Equalf(actualDecrypted, decryptedRaw, "RTP packet with SeqNum invalid decryption: %d", testCase.sequenceNumber)
	}
}

func TestRTPLifecyleInPlace(t *testing.T) {
	assert := assert.New(t)

	encryptContext, err := buildTestContext()
	if err != nil {
		t.Fatal(err)
	}

	decryptContext, err := buildTestContext()
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range rtpTestCases {
		decryptHeader := &rtp.Header{}
		decryptedPkt := &rtp.Packet{Payload: rtpTestCaseDecrypted, Header: rtp.Header{SequenceNumber: testCase.sequenceNumber}}
		decryptedRaw, err := decryptedPkt.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		encryptHeader := &rtp.Header{}
		encryptedPkt := &rtp.Packet{Payload: testCase.encrypted, Header: rtp.Header{SequenceNumber: testCase.sequenceNumber}}
		encryptedRaw, err := encryptedPkt.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		// Copy packet, asserts that everything was done in place
		encryptInput := make([]byte, len(decryptedRaw), len(decryptedRaw)+10)
		copy(encryptInput, decryptedRaw)

		actualEncrypted, err := encryptContext.EncryptRTP(encryptInput, encryptInput, encryptHeader)
		if err != nil {
			t.Fatal(err)
		} else if &encryptInput[0] != &actualEncrypted[0] {
			t.Fatal("EncryptRTP failed to encrypt in place")
		} else if encryptHeader.SequenceNumber != testCase.sequenceNumber {
			t.Fatal("EncryptRTP failed to populate input rtp.Header")
		}
		assert.Equalf(actualEncrypted, encryptedRaw, "RTP packet with SeqNum invalid encryption: %d", testCase.sequenceNumber)

		// Copy packet, asserts that everything was done in place
		decryptInput := make([]byte, len(encryptedRaw))
		copy(decryptInput, encryptedRaw)

		actualDecrypted, err := decryptContext.DecryptRTP(decryptInput, decryptInput, decryptHeader)
		if err != nil {
			t.Fatal(err)
		} else if &decryptInput[0] != &actualDecrypted[0] {
			t.Fatal("DecryptRTP failed to decrypt in place")
		} else if decryptHeader.SequenceNumber != testCase.sequenceNumber {
			t.Fatal("DecryptRTP failed to populate input rtp.Header")
		}
		assert.Equalf(actualDecrypted, decryptedRaw, "RTP packet with SeqNum invalid decryption: %d", testCase.sequenceNumber)
	}
}

func BenchmarkEncryptRTP(b *testing.B) {
	encryptContext, err := buildTestContext()
	if err != nil {
		b.Fatal(err)
	}

	pkt := &rtp.Packet{Payload: make([]byte, 100)}
	pktRaw, err := pkt.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err = encryptContext.EncryptRTP(nil, pktRaw, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncryptRTPInPlace(b *testing.B) {
	encryptContext, err := buildTestContext()
	if err != nil {
		b.Fatal(err)
	}

	pkt := &rtp.Packet{Payload: make([]byte, 100)}
	pktRaw, err := pkt.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	buf := make([]byte, 0, len(pktRaw)+10)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf, err = encryptContext.EncryptRTP(buf[:0], pktRaw, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecryptRTP(b *testing.B) {
	sequenceNumber := uint16(5000)
	encrypted := []byte{0x6d, 0xd3, 0x7e, 0xd5, 0x99, 0xb7, 0x2d, 0x28, 0xb1, 0xf3, 0xa1, 0xf0, 0xc, 0xfb, 0xfd, 0x8}

	encryptedPkt := &rtp.Packet{
		Payload: encrypted,
		Header: rtp.Header{
			SequenceNumber: sequenceNumber,
		},
	}

	encryptedRaw, err := encryptedPkt.Marshal()
	if err != nil {
		b.Fatal(err)
	}

	context, err := buildTestContext()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := context.DecryptRTP(nil, encryptedRaw, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
