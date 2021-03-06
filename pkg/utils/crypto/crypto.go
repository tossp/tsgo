package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/sm4"
	"golang.org/x/crypto/nacl/secretbox"
)

func EccEncrypt(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey, plantText []byte, salt ...byte) []byte {
	key, _ := GenerateSharedSecret(priv, pub, salt...)
	return AesEncrypt(plantText, key)
}

func EccDecrypt(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey, cipherText []byte, salt ...byte) ([]byte, error) {
	key, _ := GenerateSharedSecret(priv, pub, salt...)
	return AesDecrypt(cipherText, key)
}

func AesEncrypt(plainText, key []byte) []byte {
	k1 := HashKey(key, 32)
	block, _ := aes.NewCipher(k1) //选择加密算法
	k2 := HashKey(k1, block.BlockSize())
	plainText = Padding(plainText, block.BlockSize())
	blockModel := cipher.NewCBCEncrypter(block, k2)
	ciphertext := make([]byte, len(plainText))
	blockModel.CryptBlocks(ciphertext, plainText)
	return ciphertext
}

func AesDecrypt(cipherText, key []byte) (plantText []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			//check exactly what the panic was and create error.
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknow panic")
			}
		}
	}()

	k1 := HashKey(key, 32)
	block, _ := aes.NewCipher(k1) //选择加密算法
	k2 := HashKey(k1, block.BlockSize())
	blockModel := cipher.NewCBCDecrypter(block, k2)
	blockModel.CryptBlocks(cipherText, cipherText)
	plantText = UnPadding(cipherText)
	return
}

func Sm2Encrypt(priv *sm2.PrivateKey, pub *sm2.PublicKey, plainText []byte, salt ...byte) []byte {
	key, _ := GenerateSharedSecret(priv, pub, salt...)
	return Sm4Encrypt(plainText, key)
}

func Sm2Decrypt(priv *sm2.PrivateKey, pub *sm2.PublicKey, cipherText []byte, salt ...byte) ([]byte, error) {
	key, _ := GenerateSharedSecret(priv, pub, salt...)
	return Sm4Decrypt(cipherText, key)
}

func Sm4Encrypt(plainText, key []byte) []byte {
	k1 := HashKey(key, sm4.BlockSize)
	k2 := HashKey(k1, sm4.BlockSize)
	block, _ := sm4.NewCipher(k1)
	origData := Padding(plainText, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, k2)
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)
	return cryted
}

func Sm4Decrypt(cipherText, key []byte) (plantText []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			//check exac数据已过期tly what the panic was and create error.
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
		}
	}()
	k1 := HashKey(key, sm4.BlockSize)
	k2 := HashKey(k1, sm4.BlockSize)
	block, _ := sm4.NewCipher(k1)
	blockMode := cipher.NewCBCDecrypter(block, k2)
	//origData := make([]byte, len(cipherText))
	blockMode.CryptBlocks(cipherText, cipherText)
	plantText = UnPadding(cipherText)
	return
}

func Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:length-unpadding]
}

const (
	secretboxKeySize   = 32
	secretboxNonceSize = 24
)

var (
	errGenNonce = errors.New("could not generate enough random bytes for nonce")
	errKeyNonce = errors.New("incorrect key or nonce size")
	errDecrypt  = errors.New("decryption failed")
)

func generateNonce() ([]byte, error) {
	b, err := GenerateRandomBytes(secretboxNonceSize)
	if err != nil || len(b) != secretboxNonceSize {
		return nil, errGenNonce
	}
	return b, nil
}

func SecretboxEncrypt(input []byte, key []byte) []byte {
	nonce, _ := generateNonce()

	var nonceBytes [secretboxNonceSize]byte
	copy(nonceBytes[:], nonce)

	var keyBytes [secretboxKeySize]byte
	copy(keyBytes[:], HashKey(key, secretboxKeySize))

	return secretbox.Seal(nonce[:], input, &nonceBytes, &keyBytes)
}

/*
Decrypt Decrypt (authenticated) a series of bytes given the secret key
*/
func SecretboxDecrypt(input []byte, key []byte) (plantText []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			//check exac数据已过期tly what the panic was and create error.
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
		}
	}()
	var decryptNonce [secretboxNonceSize]byte
	copy(decryptNonce[:], input[:secretboxNonceSize])

	var keyBytes [secretboxKeySize]byte
	copy(keyBytes[:], HashKey(key, secretboxKeySize))

	cipherText := input[secretboxNonceSize:]

	decrypted, ok := secretbox.Open(nil, cipherText, &decryptNonce, &keyBytes)
	if !ok {
		return nil, errDecrypt
	}

	return decrypted, nil
}

func JsEncode(gmPriv *sm2.PrivateKey, eccPriv *ecdsa.PrivateKey, gmPub *sm2.PublicKey, eccPub *ecdsa.PublicKey, plainText string) (result map[string]string) {
	once := GetRandomString(32)
	now := time.Now().Format(time.RFC3339Nano)
	gmCipherText := Sm2Encrypt(gmPriv, gmPub, []byte(once+plainText+now), []byte(once)...)
	eccCipherText := EccEncrypt(eccPriv, eccPub, gmCipherText, []byte(once)...)
	cipherText := Base64Encode(eccCipherText)

	var (
		sign      = ""
		publicKey = fmt.Sprintf("%s|%s", Base64Encode(FromECDSAPub(&eccPriv.PublicKey)), Base64Encode(FromsSm2Pub(&gmPriv.PublicKey)))
	)
	gmSign, err := Sign2(gmPriv, []byte(now+plainText+once), nil)
	if err != nil {
		fmt.Println("警告 gmSign", err.Error())
	} else {
		sign = Base64Encode(gmSign)
	}
	eccSign, err := Sign2(eccPriv, []byte(now+plainText+once), nil)
	if err != nil {
		fmt.Println("警告 eccSign", err.Error())
		sign = fmt.Sprintf("%s|%s", sign, "")
	} else {
		sign = fmt.Sprintf("%s|%s", sign, Base64Encode(eccSign))
	}
	result = make(map[string]string)
	result["Time"] = now
	result["Once"] = once
	result["Cipher"] = cipherText
	result["Sign"] = sign
	result["PubKey"] = publicKey
	result["Hash"] = Base64Encode(GmHashKey([]byte(cipherText+once+now+sign+publicKey), 64))
	return
}

type JsDecodeHelper struct {
	Time   string
	Once   string
	Cipher string
	Sign   string
	PubKey string
	Hash   string
}

func JsDecode(gmPriv *sm2.PrivateKey, eccPriv *ecdsa.PrivateKey, opt *JsDecodeHelper) (plainText string, err error) {
	if Base64Encode(GmHashKey([]byte(opt.Cipher+opt.Once+opt.Time+opt.Sign+opt.PubKey), 64)) != opt.Hash {
		err = errors.New("数据格式错误")
		return
	}
	t, err := time.Parse(time.RFC3339Nano, opt.Time)
	if err != nil {
		err = errors.New("有效时间格式错误")
		return
	}
	if time.Now().Add(time.Minute * -1).After(t) {
		err = errors.New("数据未生效")
		return
	}
	if time.Now().Add(time.Minute * 5).Before(t) {
		err = errors.New("数据已过期")
		return
	}
	sign := strings.Split(opt.Sign, "|")
	if len(sign) != 2 {
		err = errors.New("密钥格式错误")
		return
	}

	p := strings.Split(opt.PubKey, "|")
	if len(p) != 2 {
		err = errors.New("密钥格式错误")
		return
	}
	var (
		eccPub *ecdsa.PublicKey
		gmPub  *sm2.PublicKey
	)
	if key, e := Base64Decode(p[0]); e != nil {
		err = errors.New("ECC密钥格式错误")
		return
	} else {
		eccPub = ToECDSAPub(key)
	}
	if key, e := Base64Decode(p[1]); e != nil {
		err = errors.New("ECC密钥格式错误")
		return
	} else {
		gmPub = ToSm2Pub(key)
	}

	eccCipherText, err := Base64Decode(opt.Cipher)
	if err != nil {
		err = errors.New("解码密文错误")
		return
	}
	eccPlainText, err := EccDecrypt(eccPriv, eccPub, eccCipherText, []byte(opt.Once)...)
	if err != nil {
		err = errors.New("ECC解密错误")
		return
	}
	gmPlainText, err := Sm2Decrypt(gmPriv, gmPub, eccPlainText, []byte(opt.Once)...)
	if err != nil {
		err = errors.New("GM解密错误")
		return
	}
	plainText = strings.TrimSuffix(strings.TrimPrefix(string(gmPlainText), opt.Once), opt.Time)

	s, err := Base64Decode(sign[0])
	if err != nil {
		err = errors.New("解码签名信息失败")
		return
	}
	if !gmPub.Verify([]byte(opt.Time+plainText+opt.Once), s) {
		err = errors.New("验证GM签名信息失败")
		return
	}

	s, err = Base64Decode(sign[1])
	if err != nil {
		err = errors.New("解码ECC签名信息失败")
		return
	}
	var esig struct {
		R, S *big.Int
	}
	if _, err = asn1.Unmarshal(s, &esig); err != nil {
		err = errors.New("序列化ECC签名信息失败")
		return
	}
	if !ecdsa.Verify(eccPub, []byte(opt.Time+plainText+opt.Once), esig.R, esig.S) {
		err = errors.New("验证ECC签名信息失败")
		return
	}
	return
}
