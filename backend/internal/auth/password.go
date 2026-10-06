package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword genera el hash bcrypt de una contraseña. Nunca se guardan ni
// se registran contraseñas en claro.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword compara una contraseña contra su hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyHash se usa cuando la cuenta no existe, para que un login fallido
// tarde lo mismo exista o no el email (dificulta enumerar cuentas).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("cuenta-inexistente-yampier"), bcrypt.DefaultCost)

// SpendComparisonTime ejecuta una comparación descartable con el mismo costo
// que CheckPassword.
func SpendComparisonTime(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
