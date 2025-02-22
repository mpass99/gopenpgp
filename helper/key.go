package helper

import (
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

// UpdatePrivateKeyPassphrase decrypts the given armored privateKey with oldPassphrase,
// re-encrypts it with newPassphrase, and returns the new armored key.
func UpdatePrivateKeyPassphrase(
	privateKey string,
	oldPassphrase, newPassphrase []byte,
) (string, error) {
	key, err := crypto.NewKeyFromArmored(privateKey)
	if err != nil {
		return "", fmt.Errorf("gopenpgp: unable to parse key: %w", err)
	}

	unlocked, err := key.Unlock(oldPassphrase)
	if err != nil {
		return "", fmt.Errorf("gopenpgp: unable to unlock old key: %w", err)
	}
	defer unlocked.ClearPrivateParams()

	locked, err := unlocked.Lock(newPassphrase)
	if err != nil {
		return "", fmt.Errorf("gopenpgp: unable to lock new key: %w", err)
	}

	armored, err := locked.Armor()
	if err != nil {
		return "", fmt.Errorf("gopenpgp: unable to armor new key: %w", err)
	}

	return armored, nil
}

// GenerateKey generates a key of the given keyType ("rsa" or "x25519"), encrypts it, and returns an armored string.
// If keyType is "rsa", bits is the RSA bitsize of the key.
// If keyType is "x25519" bits is unused.
func GenerateKey(name, email string, passphrase []byte, keyType string, bits int) (string, error) {
	key, err := crypto.GenerateKey(name, email, keyType, bits)
	if err != nil {
		return "", fmt.Errorf("gopenpgp: unable to generate new key: %w", err)
	}
	defer key.ClearPrivateParams()

	locked, err := key.Lock(passphrase)
	if err != nil {
		return "", fmt.Errorf("gopenpgp: unable to lock new key: %w", err)
	}

	return locked.Armor()
}

func GetSHA256Fingerprints(publicKey string) ([]string, error) {
	key, err := crypto.NewKeyFromArmored(publicKey)
	if err != nil {
		return nil, fmt.Errorf("gopenpgp: unable to parse key: %w", err)
	}

	return key.GetSHA256Fingerprints(), nil
}
