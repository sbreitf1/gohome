package jcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

var (
	base64Encoding = base64.RawURLEncoding

	defaultSalt = []byte{217, 120, 102, 168, 130, 157, 67, 162, 186, 241, 221, 193, 33, 160, 154, 231, 211, 220, 105, 8, 21, 169, 106, 229, 251, 47, 91, 152, 132, 77, 0, 128, 235, 190, 143, 171, 175, 22, 219, 38, 58, 90, 61, 246, 183, 194, 54, 151, 223, 236, 72, 12, 30, 7, 94, 200, 229, 173, 235, 104, 128, 123, 157, 113}

	// ErrChecksumMismatch is returned when the checksum of a decrypted value is wrong indicating a wrong password.
	ErrChecksumMismatch = fmt.Errorf("checksum mismatch")
)

// IsWrongPassword returns whether the given error indicates a wrong password.
func IsWrongPassword(err error) bool {
	return err == ErrChecksumMismatch
}

// Options define further parameters for marshalling and crypto operations.
type Options struct {
	// Salt defines a salt for pbkdf2 key derivation
	Salt []byte
	// GetKeyHandler is called when a key is required for marshalling or unmarshalling. Is called at most once for every operation.
	GetKeyHandler KeySource
	// YAML can be set to true to output or process YAML encoding instead of JSON.
	YAML bool
}

// KeySource defines a handler function to obtain the encryption passphrase. This handler is only called if a passphrase is required.
type KeySource func() ([]byte, error)

// StaticKey returns a KeySource to be used for Options.GetKeyHandler that simply returns a fixed key.
func StaticKey(key []byte) func() ([]byte, error) {
	return func() ([]byte, error) { return key, nil }
}

type cryptContext struct {
	key     []byte
	Options *Options
}

func newCryptContext(options *Options) *cryptContext {
	if options == nil {
		options = &Options{}
	}

	if options.YAML {
		panic("YAML not yet supported")
	}

	return &cryptContext{Options: options}
}

func (c *cryptContext) GetKey() ([]byte, error) {
	if c.key != nil {
		return c.key, nil
	}

	if c.Options.GetKeyHandler != nil {
		key, err := c.Options.GetKeyHandler()
		if err != nil {
			return nil, err
		}
		c.key = key
		return c.key, nil
	}

	return nil, fmt.Errorf("no key source defined")
}

type cryptBlock struct {
	Mode       string `json:"mode"`
	DataBase64 string `json:"data"`
	Data       []byte `json:"-"`
}

func newCryptBlock(mode string, data []byte) *cryptBlock {
	dataBase64 := base64Encoding.EncodeToString(data)
	return &cryptBlock{mode, dataBase64, data}
}

func parseCryptBlock(v interface{}) (*cryptBlock, error) {
	block, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected content %T for encrypted value", v)
	}

	rawMode, ok := block["mode"]
	if !ok {
		return nil, fmt.Errorf("encrypted block without mode")
	}
	mode, ok := rawMode.(string)
	if !ok {
		return nil, fmt.Errorf("expected mode of encrypted block to be of type string")
	}

	rawData, ok := block["data"]
	if !ok {
		return nil, fmt.Errorf("encrypted block without data")
	}
	strData, ok := rawData.(string)
	if !ok {
		return nil, fmt.Errorf("expected data of encrypted block to be of type string")
	}
	data, err := base64Encoding.DecodeString(strData)
	if err != nil {
		return nil, fmt.Errorf("expected data of encrypted block to be base64 encoded")
	}

	return &cryptBlock{mode, strData, data}, nil
}

// Marshal returns a json representation of v and replaces all jcrypt-annotated fields with encrypted values.
func Marshal(v interface{}, options *Options) ([]byte, error) {
	context := newCryptContext(options)
	return jsonMarshal(v, func(src srcValue) (interface{}, bool, error) {
		return marshalCryptHandler(src, context)
	})
}

func marshalCryptHandler(src srcValue, context *cryptContext) (interface{}, bool, error) {
	if src.StructField != nil {
		jcryptTag := strings.Split(src.StructField.Tag.Get("jcrypt"), ",")
		if len(jcryptTag[0]) > 0 {
			if jcryptTag[0] == "aes" {
				val, err := marshalCryptAES(src, context)
				if err != nil {
					return nil, false, err
				}
				return val, true, nil
			}
			return nil, false, fmt.Errorf("unknown encryption mode %q", jcryptTag[0])
		}
	}

	return nil, false, nil
}

func marshalCryptAES(src srcValue, context *cryptContext) (interface{}, error) {
	data, err := json.Marshal(src.Interface())
	if err != nil {
		return nil, err
	}

	key, err := context.GetKey()
	if err != nil {
		return nil, err
	}

	encData, err := encryptAES(data, key, context.Options.Salt)
	if err != nil {
		return nil, err
	}

	return newCryptBlock("aes", encData), nil
}

// Unmarshal reads a json-representation and decrypts all jcrypt-annoted fields.
func Unmarshal(data []byte, v interface{}, options *Options) error {
	context := newCryptContext(options)
	return jsonUnmarshal(data, v, func(src interface{}, dst dstValue) (bool, error) {
		return unmarshalCryptHandler(src, dst, context)
	})
}

func unmarshalCryptHandler(src interface{}, dst dstValue, context *cryptContext) (bool, error) {
	if dst.StructField != nil {
		jcryptTag := strings.Split(dst.StructField.Tag.Get("jcrypt"), ",")
		if len(jcryptTag[0]) > 0 {
			if jcryptTag[0] == "aes" {
				return true, unmarshalCrypt(src, dst, context)
			}
			return false, fmt.Errorf("unknown encryption mode %q", jcryptTag[0])
		}
	}

	return false, nil
}

func unmarshalCrypt(src interface{}, dst dstValue, context *cryptContext) error {
	//TODO allow raw-mode for other datatypes
	val, ok := src.(string)
	if ok {
		// raw value present
		dst.Assign(val)
		return nil
	}

	block, err := parseCryptBlock(src)
	if err != nil {
		return err
	}

	data, err := getRawDataFromCryptBlock(block, context)
	if err != nil {
		return err
	}

	var srcValue interface{}
	if err := json.Unmarshal(data, &srcValue); err != nil {
		return fmt.Errorf("failed to decode decrypted data: %v", err)
	}

	return jsonUnmarshalValue(srcValue, dst, nil)
}

func getRawDataFromCryptBlock(block *cryptBlock, context *cryptContext) ([]byte, error) {
	switch block.Mode {
	case "none":
		return block.Data, nil

	case "aes":
		key, err := context.GetKey()
		if err != nil {
			return nil, err
		}

		raw, err := decryptAES(block.Data, key, context.Options.Salt)
		if err != nil {
			return nil, err
		}
		return raw, nil

	default:
		return nil, fmt.Errorf("unknown encryption mode %q", block.Mode)
	}
}

func encryptAES(data, key, salt []byte) ([]byte, error) {
	c, err := aes.NewCipher(deriveKeyAES(key, salt))
	if err != nil {
		return nil, err
	}

	safeData := packSafeData(data, c.BlockSize())
	encData := make([]byte, c.BlockSize()+len(safeData))
	if _, err := rand.Read(encData[:c.BlockSize()]); err != nil {
		return nil, fmt.Errorf("failed to read random initialization vector: %s", err.Error())
	}

	stream := cipher.NewCFBEncrypter(c, encData[:c.BlockSize()])
	stream.XORKeyStream(encData[c.BlockSize():], safeData)

	return encData, nil
}

func decryptAES(data, key, salt []byte) ([]byte, error) {
	c, err := aes.NewCipher(deriveKeyAES(key, salt))
	if err != nil {
		return nil, err
	}

	if len(data) < c.BlockSize() {
		return nil, fmt.Errorf("data block corrupt")
	}

	decData := make([]byte, len(data)-c.BlockSize())

	stream := cipher.NewCFBDecrypter(c, data[:c.BlockSize()])
	stream.XORKeyStream(decData, data[c.BlockSize():])

	return unpackSafeData(decData, c.BlockSize())
}

func deriveKeyAES(key, salt []byte) []byte {
	if key == nil {
		key = []byte{}
	}
	if salt == nil {
		salt = defaultSalt
	}

	return pbkdf2.Key(key, salt, 4096, 32, sha256.New)
}

func packSafeData(data []byte, blockSize int) []byte {
	h := sha256.New()
	h.Write(data)
	hash := h.Sum(nil)
	actualLen := 32 + 4 + len(data)
	padding := getPadding(actualLen, blockSize)
	safeData := make([]byte, actualLen+padding)
	copy(safeData[:32], hash)
	binary.LittleEndian.PutUint32(safeData[32:36], uint32(len(data)))
	copy(safeData[36:], data)
	rand.Read(safeData[actualLen:])
	return safeData
}

func unpackSafeData(safeData []byte, blockSize int) ([]byte, error) {
	if len(safeData) < 36 {
		return nil, fmt.Errorf("data block corrupt")
	}

	dataLen := binary.LittleEndian.Uint32(safeData[32:36])
	if int(dataLen) > (len(safeData) - 36) {
		return nil, ErrChecksumMismatch
	}

	h := sha256.New()
	h.Write(safeData[36 : 36+dataLen])
	hash := h.Sum(nil)
	for i := 0; i < 32; i++ {
		if safeData[i] != hash[i] {
			return nil, ErrChecksumMismatch
		}
	}

	return safeData[36 : 36+dataLen], nil
}

func getPadding(dataLen, blockSize int) int {
	blockCount := dataLen / blockSize
	if (blockCount * blockSize) > dataLen {
		return (blockCount * blockSize) - dataLen
	} else if (blockCount * blockSize) < dataLen {
		return (blockCount*blockSize + blockSize) - dataLen
	}
	return 0
}
