//go:build !ios && !android
// +build !ios,!android

package helper

import (
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

// EncryptSignAttachment encrypts an attachment using a detached signature, given a publicKey, a privateKey
// and its passphrase, the filename, and the unencrypted file data.
// Returns keypacket, dataPacket and unarmored (!) signature separate.
func EncryptSignAttachment(
	publicKey, privateKey string, passphrase []byte, filename string, plainData []byte,
) (keyPacket, dataPacket, signature []byte, err error) {
	var privateKeyObj, unlockedKeyObj *crypto.Key
	var publicKeyRing, privateKeyRing *crypto.KeyRing
	var packets *crypto.PGPSplitMessage
	var signatureObj *crypto.PGPSignature

	var binMessage = crypto.NewPlainMessageFromFile(plainData, filename, uint32(crypto.GetUnixTime()))

	if publicKeyRing, err = createPublicKeyRing(publicKey); err != nil {
		return nil, nil, nil, err
	}

	if privateKeyObj, err = crypto.NewKeyFromArmored(privateKey); err != nil {
		return nil, nil, nil, fmt.Errorf("gopenpgp: unable to parse private key: %w", err)
	}

	if unlockedKeyObj, err = privateKeyObj.Unlock(passphrase); err != nil {
		return nil, nil, nil, fmt.Errorf("gopenpgp: unable to unlock key: %w", err)
	}
	defer unlockedKeyObj.ClearPrivateParams()

	if privateKeyRing, err = crypto.NewKeyRing(unlockedKeyObj); err != nil {
		return nil, nil, nil, fmt.Errorf("gopenpgp: unable to create private keyring: %w", err)
	}

	if packets, err = publicKeyRing.EncryptAttachment(binMessage, ""); err != nil {
		return nil, nil, nil, fmt.Errorf("gopenpgp: unable to encrypt attachment: %w", err)
	}

	if signatureObj, err = privateKeyRing.SignDetached(binMessage); err != nil {
		return nil, nil, nil, fmt.Errorf("gopenpgp: unable to sign attachment: %w", err)
	}

	return packets.GetBinaryKeyPacket(), packets.GetBinaryDataPacket(), signatureObj.GetBinary(), nil
}

// EncryptSignArmoredDetached takes a public key for encryption,
// a private key and its passphrase for signature, and the plaintext data
// Returns an armored ciphertext and a detached armored signature.
func EncryptSignArmoredDetached(
	publicKey, privateKey string,
	passphrase, plainData []byte,
) (ciphertextArmored, encryptedSignatureArmored string, err error) {
	return encryptSignArmoredDetached(publicKey, privateKey, passphrase, plainData)
}

// EncryptSignBinaryDetached takes a public key for encryption,
// a private key and its passphrase for signature, and the plaintext data
// Returns encrypted binary data and a detached armored encrypted signature.
func EncryptSignBinaryDetached(
	publicKey, privateKey string,
	passphrase, plainData []byte,
) (encryptedData []byte, encryptedSignatureArmored string, err error) {
	return encryptSignBinaryDetached(publicKey, privateKey, passphrase, plainData)
}
