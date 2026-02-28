package defaultclient

import "net/http"

func Negative(url string) error {
	client := &http.Client{}
	_, err := client.Get(url)
	return err
}
