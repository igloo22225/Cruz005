package main

import (
	"bufio"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/skratchdot/open-golang/open"
)

var debugMode = false

func introText() {
	fmt.Println("********** PLEASE READ BELOW BEFORE CONTINUING **********") //Give the user an overview and the obligatory sec warning.
	fmt.Println("Cruz005 - A Cruz ID MFA that poses no immediate risk in any direct sense.")
	fmt.Println("Cruz005 will take a Duo activation URL and will generate a QR code for use with other 2-factor authentication apps.")
	fmt.Println("Cruz005 will store the token as an image on the disk. You will be prompted to delete it at the end.")
	fmt.Println("Now then -")
	fmt.Println("Browse to the CruzKey settings page and set up a new device. Have it email you a URL.")
	fmt.Println("You will use that URL to establish a code.")
	fmt.Println("When asked, this is an Android device you are registering.")
	fmt.Println()
	fmt.Print("URL: ")
}

func getActivationCode(url string) string {
	location := strings.Index(url, "android/") //Look through the URL to find where the token should be, pull it and return it
	location = location + 8                     //While not the most graceful at handling errors, we have already checked for the most common URL failures
	activationToken := string(url[location : location+20])
	return activationToken
}

func registerAsClient(activationToken string) string {
	type DuoResponse struct {
		Response struct {
			Akey                   string  `json:"akey"`
			AppStatus              float64 `json:"app_status"`
			CurrentAppVersion      string  `json:"current_app_version"`
			CurrentOsVersion       string  `json:"current_os_version"`
			CustomerName           string  `json:"customer_name"`
			ForceDisableAnalytics  bool    `json:"force_disable_analytics"`
			HasBackupRestore       bool    `json:"has_backup_restore"`
			HasBluetoothApprove    bool    `json:"has_bluetooth_approve"`
			HasDeviceInsight       bool    `json:"has_device_insight"`
			HasTrustedEndpoints    bool    `json:"has_trusted_endpoints"`
			HotpSecret             string  `json:"hotp_secret"`
			IsFipsDeployment       bool    `json:"is_fips_deployment"`
			OsStatus               float64 `json:"os_status"`
			Pkey                   string  `json:"pkey"`
			ReactivationToken      string  `json:"reactivation_token"`
			RequiresFipsAndroid    bool    `json:"requires_fips_android"`
			RequiresMdm            float64 `json:"requires_mdm"`
			SecurityCheckupEnabled bool    `json:"security_checkup_enabled"`
			UrgSecret              string  `json:"urg_secret"`
		} `json:"response"`
		Stat string `json:"stat"`
	}
	client := &http.Client{}
	data := url.Values{}
	data.Set("app_id", "com.duosecurity.duomobile.app.DMApplication") //This data has to be sent as post data as Duo doesn't want to give it to just any device
	data.Set("app_version", "2.3.3")
	data.Set("app_build_number", "323206")
	data.Set("full_disk_encryption", "false")
	data.Set("manufacturer", "Google")
	data.Set("model", "Pixel") //Ah yes, Golang Pixel!
	data.Set("platform", "Android")
	data.Set("jailbroken", "False")
	data.Set("version", "6.0")
	data.Set("language", "EN")
	data.Set("customer_protocol", "1")
	fullURL := "https://api-268194b0.duosecurity.com/push/v2/activation/" + activationToken
	if debugMode == true {
		fmt.Println("DUO Request URL: " + fullURL)
	}
	req, _ := http.NewRequest("POST", fullURL, strings.NewReader(data.Encode())) //Emulate the app and request the token
	req.Header.Set("User-Agent", "okhttp/3.11.0")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	if debugMode == true {
		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Headers:", resp.Header)
		fmt.Println("response Body:", string(body))
	}
	var DecodedResp DuoResponse
	error := json.Unmarshal(body, &DecodedResp)
	if error != nil {
		fmt.Println("JSON parsing error, check Duo's JSON response.")
		fmt.Println("response Body:", string(body))
		panic(err)
	}
	if debugMode == true {
		fmt.Println("Response: " + DecodedResp.Stat)
		fmt.Println("Token: " + DecodedResp.Response.HotpSecret)
	}
	HOTPToken := DecodedResp.Response.HotpSecret
	return HOTPToken
}

func validateURL(urlIn string) bool {
	_, err := url.ParseRequestURI(urlIn)
	if err != nil {
		return false
	}

	match, _ := regexp.MatchString(`https:\/\/m-268194b0\.duosecurity.com\/android\/[a-zA-Z0-9]+`, urlIn)
	if !match {
		fmt.Println("That doesn't look like a CruzID MFA URL! Please restart and try again.")
		return false
	}

	return true
}

func getDuoData() string {
	url := "" //Take the user's URL
	reader := bufio.NewReader(os.Stdin)
	url, err := reader.ReadString('\n')
	url = strings.TrimSpace(url)

	if err != nil {
		fmt.Println("Error reading URL, please restart.") //If for whatever reason the string isn't readable, yell
		os.Exit(1)
	}
	if validateURL(url) == false {
		fmt.Println("URL looks wrong! Please restart.")
		os.Exit(1)
	}
	activationToken := getActivationCode(url)
	if debugMode == true {
		fmt.Println("Your activation token appears to be " + activationToken + ".")
	}
	fmt.Println("Attempting to register with Duo...")
	HOTPToken := registerAsClient(activationToken)
	return HOTPToken
}

func generateQRCode(HOTPToken string) {
	fmt.Println("Generating QR Code...")
	if HOTPToken == "" {
		fmt.Println("Error: HOTP Token not present. Something went wrong.")
		os.Exit(1)
	}
	QRCode := base32.StdEncoding.EncodeToString([]byte(HOTPToken))
	QRCode = "otpauth://hotp/Cruz_ID_MFA?secret=" + QRCode
	err := qrcode.WriteFile(QRCode, qrcode.Medium, 256, "bk.png")
	if err != nil {
		panic(err)
	}
	fmt.Println("Opening QR code using your default photo viewer...")
	open.Run("bk.png")
}

func cleanup() {
	fmt.Println("Please scan the QR code with your 2-factor application.")
	fmt.Println("Once you have done so, you should delete the image from local storage (in order to protect your account).")
	fmt.Println("Before continuing, please close the local program displaying your image.")
	fmt.Println("To do so, simply hit enter. If you would rather keep the image (a *really* bad idea unless you protect it), enter \"n\", and press enter.")
	selection := "" //Take the user's URL
	reader := bufio.NewReader(os.Stdin)
	selection, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading selection, please restart.") //If for whatever reason the string isn't readable, yell
		return
	}
	selection = strings.ToLower(strings.Trim(selection, " \r\n"))
	if selection == "n" {
		fmt.Println("Image has not been removed! Please remember to secure it!")
	} else {
		os.Remove("bk.png")
		fmt.Println("Image has been removed.")
	}
	fmt.Println("Program complete.")
}

func main() {
	//Intro text
	introText()
	//Take input, go get json, find the HOTP token
	HOTPToken := getDuoData()
	//Generate a QR code with the token
	generateQRCode(HOTPToken)
	//Wait for the user to confirm cleanup
	cleanup()

}
