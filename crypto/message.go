package crypto

import (
	"bytes"
	"errors"
	"io/ioutil"
	"time"

	armorUtils "github.com/ProtonMail/go-pm-crypto/armor"
	"github.com/ProtonMail/go-pm-crypto/internal"
	"github.com/ProtonMail/go-pm-crypto/models"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	errors2 "golang.org/x/crypto/openpgp/errors"
	"golang.org/x/crypto/openpgp/packet"
	"math"
)

// DecryptMessage decrypt encrypted message use private key (string )
// encryptedText : string armored encrypted
// privateKey : armored private use to decrypt message
// passphrase : match with private key to decrypt message
func (pm *PmCrypto) DecryptMessageStringKey(encryptedText string, privateKey string, passphrase string) (string, error) {
	privKeyRaw, err := armorUtils.Unarmor(privateKey)
	if err != nil {
		return "", err
	}
	privKeyReader := bytes.NewReader(privKeyRaw)
	privKeyEntries, err := openpgp.ReadKeyRing(privKeyReader)
	if err != nil {
		return "", err
	}

	return pm.DecryptMessage(encryptedText, &KeyRing{entities: privKeyEntries}, passphrase)
}

// DecryptMessageBinKey decrypt encrypted message use private key (bytes )
// encryptedText : string armored encrypted
// privateKey : unarmored private use to decrypt message could be mutiple keys
// passphrase : match with private key to decrypt message
func (kr *KeyRing) DecryptMessage(encryptedText string, passphrase string) (string, error) {

	md, err := pm.decryptCore(encryptedText, nil, kr.entities, passphrase, pm.getTimeGenerator())
	if err != nil {
		return "", err
	}

	decrypted := md.UnverifiedBody
	b, err := ioutil.ReadAll(decrypted)
	if err != nil {
		return "", err
	}

	println(4)
	return string(b), nil
}

// DecryptMessageVerifyBinKeyPrivBinKeys decrypt message and verify the signature
// verifierKey []byte: unarmored verifier keys
// privateKey []byte: unarmored private key to decrypt. could be mutiple
func (pm *PmCrypto) DecryptMessageVerify(encryptedText string, verifierKey []byte, privateKeysRing *KeyRing, passphrase string, verifyTime int64) (*models.DecryptSignedVerify, error) {
	return pm.decryptMessageVerify(encryptedText, verifierKey, privateKeysRing, passphrase, verifyTime)
}

func (pm *PmCrypto) decryptCore(encryptedText string, additionalEntries openpgp.EntityList, privKeyEntries openpgp.EntityList, passphrase string, timeFunc func() time.Time) (*openpgp.MessageDetails, error) {

	rawPwd := []byte(passphrase)
	for _, e := range privKeyEntries {

		if e.PrivateKey != nil && e.PrivateKey.Encrypted {
			e.PrivateKey.Decrypt(rawPwd)
		}

		for _, sub := range e.Subkeys {
			if sub.PrivateKey != nil && sub.PrivateKey.Encrypted {
				sub.PrivateKey.Decrypt(rawPwd)
			}
		}
	}

	if additionalEntries != nil {
		for _, e := range additionalEntries {
			privKeyEntries = append(privKeyEntries, e)
		}
	}

	encryptedio, err := internal.Unarmor(encryptedText)
	if err != nil {
		return nil, err
	}

	config := &packet.Config{Time: timeFunc}

	md, err := openpgp.ReadMessage(encryptedio.Body, privKeyEntries, nil, config)
	return md, err
}

// decryptMessageVerify
// decrypt_message_verify_single_key(private_key: string, passphras: string, encrypted : string, signature : string) : decrypt_sign_verify;
// decrypt_message_verify(passphras: string, encrypted : string, signature : string) : decrypt_sign_verify;
func (pm *PmCrypto) decryptMessageVerify(encryptedText string, verifierKey []byte, privateKeyRing *KeyRing, passphrase string, verifyTime int64) (*models.DecryptSignedVerify, error) {

	out := &models.DecryptSignedVerify{}
	out.Verify = failed

	var verifierEntries openpgp.EntityList
	if len(verifierKey) > 0 {
		verifierReader := bytes.NewReader(verifierKey)
		var err error
		verifierEntries, err = openpgp.ReadKeyRing(verifierReader)
		if err != nil {
			return nil, err
		}

	} else {
		out.Verify = noVerifier
	}

	md, err := pm.decryptCore(encryptedText, verifierEntries, privateKeyRing.entities, passphrase, func() time.Time { return time.Unix(0, 0) }) // TODO: I doubt this time is correct

	decrypted := md.UnverifiedBody
	b, err := ioutil.ReadAll(decrypted)
	if err != nil {
		return nil, err
	}

	processSignatureExpiration(md, verifyTime)

	out.Plaintext = string(b)
	if md.IsSigned {
		if md.SignedBy != nil {
			if verifierEntries != nil {
				matches := verifierEntries.KeysById(md.SignedByKeyId)
				if len(matches) > 0 {
					if md.SignatureError == nil {
						out.Verify = ok
					} else {
						out.Message = md.SignatureError.Error()
						out.Verify = failed
					}
				}
			} else {
				out.Verify = noVerifier
			}
		} else {
			out.Verify = noVerifier
		}
	} else {
		out.Verify = notSigned
	}
	return out, nil
}

// Handle signature time verification manually, so we can add a margin to the creationTime check.
func processSignatureExpiration(md *openpgp.MessageDetails, verifyTime int64) {
	if md.SignatureError == errors2.ErrSignatureExpired {
		if verifyTime > 0 {
			created := md.Signature.CreationTime.Unix()
			expires := int64(math.MaxInt64)
			if md.Signature.KeyLifetimeSecs != nil {
				expires = int64(*md.Signature.KeyLifetimeSecs) + created
			}
			if created-internal.CreationTimeOffset <= verifyTime && verifyTime <= expires {
				md.SignatureError = nil
			}
		} else {
			// verifyTime = 0: time check disabled, everything is okay
			md.SignatureError = nil
		}
	}
}

//EncryptMessageWithPassword encrypt a plain text to pgp message with a password
//plainText string: clear text
//output string: armored pgp message
func (pm *PmCrypto) EncryptMessageWithPassword(plainText string, password string) (string, error) {

	var outBuf bytes.Buffer
	w, err := armor.Encode(&outBuf, armorUtils.PGP_MESSAGE_HEADER, internal.ArmorHeaders)
	if err != nil {
		return "", err
	}

	config := &packet.Config{Time: pm.getTimeGenerator()}
	plaintext, err := openpgp.SymmetricallyEncrypt(w, []byte(password), nil, config)
	if err != nil {
		return "", err
	}
	message := []byte(plainText)
	_, err = plaintext.Write(message)
	if err != nil {
		return "", err
	}
	err = plaintext.Close()
	if err != nil {
		return "", err
	}
	w.Close()

	return outBuf.String(), nil
}

// EncryptMessageBinKey encrypt message with unarmored public key, if pass private key and passphrase will also sign the message
// publicKey : bytes unarmored public key
// plainText : the input
// privateKey : optional required when you want to sign
// passphrase : optional required when you pass the private key and this passphrase must could decrypt the private key
func (pm *PmCrypto) EncryptMessage(plainText string, publicKey *KeyRing, privateKey *KeyRing, passphrase string, trim bool) (string, error) {

	if trim {
		plainText = internal.TrimNewlines(plainText)
	}
	var outBuf bytes.Buffer
	w, err := armor.Encode(&outBuf, armorUtils.PGP_MESSAGE_HEADER, internal.ArmorHeaders)
	if err != nil {
		return "", err
	}

	var signEntity *openpgp.Entity

	if len(passphrase) > 0 && len(privateKey.entities) > 0 {

		for _, e := range privateKey.entities {
			// Entity.PrivateKey must be a signing key
			if e.PrivateKey != nil {
				if e.PrivateKey.Encrypted {
					e.PrivateKey.Decrypt([]byte(passphrase))
				}
				if !e.PrivateKey.Encrypted {
					signEntity = e
					break
				}
			}
		}

		if signEntity == nil {
			return "", errors.New("cannot sign message, signer key is not unlocked")
		}
	}

	ew, err := EncryptCore(w, publicKey.entities, signEntity, "", false, pm.getTimeGenerator())

	_, _ = ew.Write([]byte(plainText))
	ew.Close()
	w.Close()
	return outBuf.String(), nil
}
