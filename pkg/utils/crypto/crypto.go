package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/sm4"
	"github.com/tossp/tsgo/pkg/utils"
)

func EccEncrypt(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey, plantText []byte) []byte {
	return AesEncrypt(plantText, GenerateSharedSecret(priv, pub))
}

func EccDecrypt(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey, cipherText []byte) ([]byte, error) {
	return AesDecrypt(cipherText, GenerateSharedSecret(priv, pub))
}

func AesEncrypt(plainText, key []byte) []byte {
	k := HashKey(key, 32)
	block, _ := aes.NewCipher(k) //选择加密算法
	plainText = Padding(plainText, block.BlockSize())
	blockModel := cipher.NewCBCEncrypter(block, k[:block.BlockSize()])
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

	k := HashKey(key, 32)
	block, _ := aes.NewCipher(k) //选择加密算法
	blockModel := cipher.NewCBCDecrypter(block, k[:block.BlockSize()])
	blockModel.CryptBlocks(cipherText, cipherText)
	plantText = UnPadding(cipherText)
	return
}

func Sm2Encrypt(priv *sm2.PrivateKey, pub *sm2.PublicKey, plainText []byte) []byte {
	return Sm4Encrypt(plainText, GenerateSm2SharedSecret(priv, pub))
}

func Sm2Decrypt(priv *sm2.PrivateKey, pub *sm2.PublicKey, cipherText []byte) ([]byte, error) {
	return Sm4Decrypt(cipherText, GenerateSm2SharedSecret(priv, pub))
}

func Sm4Encrypt(plainText, key []byte) []byte {
	k := HashKey(key, sm4.BlockSize*2)
	block, _ := sm4.NewCipher(k[:sm4.BlockSize])
	origData := Padding(plainText, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, k[sm4.BlockSize:sm4.BlockSize+block.BlockSize()])
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)
	return cryted
}

func Sm4Decrypt(cipherText, key []byte) ([]byte, error) {
	k := HashKey(key, sm4.BlockSize*2)
	block, _ := sm4.NewCipher(k[:sm4.BlockSize])
	blockMode := cipher.NewCBCDecrypter(block, k[sm4.BlockSize:sm4.BlockSize+block.BlockSize()])
	origData := make([]byte, len(cipherText))
	blockMode.CryptBlocks(origData, cipherText)
	origData = UnPadding(origData)
	return origData, nil
}

func Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func UnPadding(plantText []byte) []byte {
	length := len(plantText)
	unpadding := int(plantText[length-1])
	return plantText[:(length - unpadding)]
}

func JsEncode(gmPriv *sm2.PrivateKey, eccPriv *ecdsa.PrivateKey, gmPub *sm2.PublicKey, eccPub *ecdsa.PublicKey, plainText string) (result map[string]string) {
	once := utils.GetRandomString(32)
	now := time.Now().Format(time.RFC3339Nano)
	gmCipherText := Sm2Encrypt(gmPriv, gmPub, []byte(once+plainText+now))
	eccCipherText := EccEncrypt(eccPriv, eccPub, gmCipherText)
	//fmt.Println("GM密文", len(gmCipherText), Base64Encode(gmCipherText))
	cipherText := Base64Encode(eccCipherText)
	//fmt.Println("ECC密文", len(eccCipherText), cipherText)

	//eccCipherText, _ = Base64Decode(cipherText)
	//eccPlainText, _ := EccDecrypt(eccPriv, eccPub, eccCipherText)
	//gmPlainText, _ := Sm2Decrypt(gmPriv, gmPub, eccPlainText)
	//fmt.Println("GM明文", len(gmPlainText), string(gmPlainText))

	var (
		sign      = ""
		publicKey = fmt.Sprintf("%s|%s", Base64Encode(FromECDSAPub(&eccPriv.PublicKey)), Base64Encode(FromsSm2Pub(&gmPriv.PublicKey)))
	)
	gmSign, err := Sign2(gmPriv, []byte(once+cipherText+now))
	if err != nil {
		fmt.Println("警告 gmSign", err.Error())
	} else {
		sign = Base64Encode(gmSign)
	}
	eccSign, err := Sign2(eccPriv, []byte(once+cipherText+now))
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
	now := time.Now()
	fmt.Println("JsDecode", now)
	fmt.Println("JsDecode", now.Add(time.Minute*5))
	fmt.Println("JsDecode", t)
	fmt.Println("JsDecode", t.Sub(now))
	if now.Before(t) {
		err = errors.New("数据未生效")
		return
	}

	if t.Sub(now) >= time.Minute*3 {
		err = errors.New("数据已过期")
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
	eccPlainText, err := EccDecrypt(eccPriv, eccPub, eccCipherText)
	if err != nil {
		err = errors.New("ECC解密错误")
		return
	}
	gmPlainText, err := Sm2Decrypt(gmPriv, gmPub, eccPlainText)
	if err != nil {
		err = errors.New("GM解密错误")
		return
	}
	plainText = strings.TrimSuffix(strings.TrimPrefix(string(gmPlainText), opt.Once), opt.Time)
	return
}
