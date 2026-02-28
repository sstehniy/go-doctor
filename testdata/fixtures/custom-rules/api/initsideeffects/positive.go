package initsideeffects

import "os"

func init() {
	_ = os.Setenv("APP_MODE", "dev") // want api/init-side-effects
}
