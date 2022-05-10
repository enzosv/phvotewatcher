package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type RapplerResult struct {
	SelfLink        string      `json:"self_link"`
	PoliticianLink  string      `json:"politician_link"`
	ID              int         `json:"id"`
	BallotNo        string      `json:"ballot_no"`
	RawName         string      `json:"raw_name"`
	FirstName       string      `json:"first_name"`
	AlternateName   string      `json:"alternate_name"`
	MiddleName      string      `json:"middle_name"`
	LastName        string      `json:"last_name"`
	Suffix          interface{} `json:"suffix"`
	PlaceholderName string      `json:"placeholder_name"`
	Gender          string      `json:"gender"`
	Thumbnail       string      `json:"thumbnail"`
	WpSlug          string      `json:"wp_slug"`
	PoliticalParty  string      `json:"political_party"`
	Contest         struct {
		SelfLink     string      `json:"self_link"`
		ID           int         `json:"id"`
		Name         string      `json:"name"`
		DisplayName  string      `json:"display_name"`
		Code         string      `json:"code"`
		Scope        string      `json:"scope"`
		ElectionYear int         `json:"election_year"`
		District     interface{} `json:"district"`
		City         interface{} `json:"city"`
		Province     interface{} `json:"province"`
		Location     interface{} `json:"location"`
	} `json:"contest"`
	ContestName    string  `json:"contest_name"`
	Selected       bool    `json:"selected"`
	VoteCount      string  `json:"voteCount"`
	VotePercentage float64 `json:"votePercentage"`
	Color          string  `json:"color"`
	Legend         string  `json:"legend"`
}
type RapplerResponse struct {
	PageProps struct {
		Presidential struct {
			Results []RapplerResult `json:"results"`
		} `json:"presidential"`
		VPresidential struct {
			Results []RapplerResult `json:"results"`
		} `json:"vPresidential"`
		RegionList struct {
			Count    int         `json:"count"`
			Next     interface{} `json:"next"`
			Previous interface{} `json:"previous"`
			Results  []struct {
				SelfLink    string `json:"self_link"`
				ID          int    `json:"id"`
				Name        string `json:"name"`
				DisplayName string `json:"display_name"`
				Psgc        int    `json:"psgc"`
				Slug        string `json:"slug"`
				Provinces   []struct {
					SelfLink    string `json:"self_link"`
					ID          int    `json:"id"`
					Name        string `json:"name"`
					DisplayName string `json:"display_name"`
					Psgc        int    `json:"psgc"`
					Slug        string `json:"slug"`
				} `json:"provinces"`
			} `json:"results"`
		} `json:"regionList"`
	} `json:"pageProps"`
}

type GMAResponse struct {
	LocationCode string `json:"location_code"`
	Result       []struct {
		Contest    string `json:"contest"`
		Candidates []struct {
			Name      string `json:"name"`
			VoteCount int    `json:"vote_count"`
			Party     string `json:"party"`
		} `json:"candidates"`
	} `json:"result"`
	ElectionReturnsProcessed string `json:"election_returns_processed"`
	TotalVotersProcessed     string `json:"total_voters_processed"`
	ServerLocation           string `json:"server_location"`
	ResultAsOf               string `json:"result_as_of"`
}

type Storage struct {
	Lead      int     `json:"lead"`
	Processed float64 `json:"processed"`
}

type Config struct {
	BotID     string `json:"bot_id"`
	Recipient string `json:"recipient"`
}

const SOURCE = "https://e22c.gmanetwork.com/n/PRESIDENT_PHILIPPINES.json"

// const SOURCE = "https://ph.rappler.com/_next/data/81096af9b5567f2c2e380c77f98f1c7439fc4a3c/en/elections/2022/races/president-vice-president/results.json"
const TGURL = "https://api.telegram.org"

func main() {
	configPath := flag.String("c", "config.json", "config file")
	oldPath := flag.String("o", "old.json", "persistence file")
	flag.Parse()

	config, err := parseConfig(*configPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	storage, err := gmaFetch()
	if err != nil {
		fmt.Println(err)
		return
	}
	old, err := load(*oldPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	save(*oldPath, storage)

	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(old.Processed, storage.Processed)
	if old.Processed == storage.Processed {
		// no dif
		return
	}
	pdif := float64(old.Lead-storage.Lead) * 100 / float64(storage.Lead)
	fmt.Println(pdif)
	reported := storage.Processed * 100
	if err != nil {
		fmt.Println(err)
		return
	}
	modifier := ""
	if pdif < 0 {
		modifier = "`"
	}
	if pdif > 0 {
		modifier = "*"
	}
	p := message.NewPrinter(language.English)
	lead := p.Sprintf("%s%d (%.2f%%)%s", modifier, storage.Lead, pdif, modifier)
	message := fmt.Sprintf("Lead: %s\nProcessed: %.2f%%\nRemaining: %.2f%%", lead, reported, 100-reported)
	fmt.Println(message)
	sendMessage(config.BotID, config.Recipient, message)
}

func gmaFetch() (Storage, error) {
	req, err := http.NewRequest("GET", SOURCE, nil)
	if err != nil {
		return Storage{}, err
	}
	req.Header.Add("referer", "https://www.gmanetwork.com/")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Storage{}, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return Storage{}, err
	}
	var response GMAResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return Storage{}, err
	}
	leni := 0
	opposer := 0
	for _, r := range response.Result {
		for _, c := range r.Candidates {
			if c.Name == "ROBREDO, LENI (IND)" {
				leni = c.VoteCount
				continue
			}
			if c.VoteCount > opposer {
				opposer = c.VoteCount
			}
		}
	}
	recorded := strings.Split(response.ElectionReturnsProcessed, "/")
	rCount, err := strconv.ParseFloat(recorded[0], 64)
	if err != nil {
		return Storage{}, err
	}
	rTotal, err := strconv.ParseFloat(recorded[1], 64)
	if err != nil {
		return Storage{}, err
	}
	processed := rCount / rTotal
	return Storage{leni - opposer, float64(processed)}, nil
}

func fetch() ([]RapplerResult, error) {
	req, err := http.NewRequest("GET", SOURCE, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("referer", "https://www.gmanetwork.com/")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var response RapplerResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	return response.PageProps.Presidential.Results, nil
}

func save(path string, storage Storage) error {
	latest, err := json.Marshal(storage)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, latest, 0644)
}

func load(path string) (Storage, error) {
	var storage Storage
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return storage, err
	}
	err = json.Unmarshal(content, &storage)
	return storage, err
}

func parseConfig(path string) (Config, error) {
	var config Config
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(content, &config)
	return config, err
}

// telegram
func constructPayload(chatID, message string) (*bytes.Reader, error) {
	payload := map[string]interface{}{}
	payload["chat_id"] = chatID
	payload["text"] = message
	payload["parse_mode"] = "markdown"
	payload["disable_web_page_preview"] = true

	jsonValue, err := json.Marshal(payload)
	return bytes.NewReader(jsonValue), err
}

func sendMessage(bot, chatID, message string) error {
	payload, err := constructPayload(chatID, message)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/bot%s/sendMessage", TGURL, bot), payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	_, err = http.DefaultClient.Do(req)
	return err
}
