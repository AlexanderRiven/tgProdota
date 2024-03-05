package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type PageJson struct {
	Name          string `json:"name"`
	Headline      string `json:"headline"`
	Text          string `json:"text"`
	DateCreated   string `json:"dateCreated"`
	DatePublished string `json:"datePublished"`
	PageStart     int    `json:"pageStart"`
	PageEnd       int    `json:"pageEnd"`
	Image         string `json:"image"`
	Author        struct {
		URL string `json:"url"`
	} `json:"author"`
	InteractionStatistic []struct {
		Type                 string `json:"@type"`
		InteractionType      string `json:"interactionType"`
		UserInteractionCount int    `json:"userInteractionCount"`
	} `json:"interactionStatistic"`
	Context  string `json:"@context"`
	Type     string `json:"@type"`
	ID       string `json:"@id"`
	IsPartOf struct {
		ID string `json:"@id"`
	} `json:"isPartOf"`
	URL           string    `json:"url"`
	DiscussionURL string    `json:"discussionUrl"`
	Comments      []Comment `json:"comment"`
}

type Comment struct {
	Type   string `json:"@type"`
	ID     string `json:"@id"`
	URL    string `json:"url"`
	Author struct {
		Type  string `json:"@type"`
		Name  string `json:"name"`
		Image string `json:"image"`
		URL   string `json:"url"`
	} `json:"author"`
	DateCreated string `json:"dateCreated"`
	Text        string `json:"text"`
}

type Link struct {
	Url  string
	Type string
}

//type User struct {
//	id             int64
//	waitForPageNum bool
//	lastCommand    string
//	lastRead       map[Link]int
//}

// var blackList = make(map[string][]string)
var lastCommandFromUser = make(map[int64]string)
var waitForPageFromUser = make(map[int64]bool)
var linksMap = make(map[string]Link)

// Define the regular expression patterns
var subForumPattern = `forum/(\d+)`
var topicPattern = `topic/(\d+)`
var pagePattern = `page/(\d+)`

// Compile the regular expressions
var subForumRe = regexp.MustCompile(subForumPattern)
var topicRe = regexp.MustCompile(topicPattern)
var pageRe = regexp.MustCompile(pagePattern)

// GetLinkType Get type of forum page
func GetLinkType(link string) string {
	subForumPattern := `forum/(\d+)(/page/\d+)?/$`
	threadPattern := `topic/(\d+)(/page/\d+)?/$`
	mainPagePattern := `^https://prodota.ru/forum/$`

	if match, _ := regexp.MatchString(subForumPattern, link); match {
		return "SubForum"
	} else if match, _ := regexp.MatchString(threadPattern, link); match {
		return "Thread"
	} else if match, _ := regexp.MatchString(mainPagePattern, link); match {
		return "MainPage"
	}

	return "Unknown"
}

// AddLinkToMap Store shortcuts to page links in map
func AddLinkToMap(name string, url string) {
	name = strings.TrimSpace(name)
	if _, ok := linksMap[name]; !ok {
		linksMap[name] = Link{
			Url:  url,
			Type: GetLinkType(url),
		}
	}
}

// PrettyPrintString Format posts to better view
func PrettyPrintString(input string) string {
	// Replace multiple consecutive \n with a single \n
	re := regexp.MustCompile(`\n+`)
	noExtraNewlines := re.ReplaceAllString(input, "\n")

	// Remove consecutive whitespace characters after \n
	re = regexp.MustCompile(`\n\s*`)
	noExtraSpaces := re.ReplaceAllString(noExtraNewlines, "\n")

	// Trim leading/trailing whitespace
	trimmed := strings.TrimSpace(noExtraSpaces)

	return trimmed
}

// GetSubForumsFromMainPage Get list of subforums from main page
func GetSubForumsFromMainPage(body string) map[string]string {
	pattern := `<a href="(https://prodota.ru/forum/\d+/)">([^<]+)</a>`
	subForums := make(map[string]string)
	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Find all matches in the large string
	matches := re.FindAllStringSubmatch(body, -1)

	// Iterate over the matches and print the desired information
	for _, match := range matches {

		url := match[1]
		text := match[2]
		subForums[text] = url
	}
	return subForums
}

// GetThreadsFromSubForums Get list of threads from subforum page
func GetThreadsFromSubForums(body string) map[string]string {
	pattern := `<a href="(https://prodota.ru/forum/topic/\d+/\?do=getNewComment)" class="" title="([^"]+)"`
	threads := make(map[string]string)
	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Find all matches in the large string
	matches := re.FindAllStringSubmatch(body, -1)

	// Iterate over the matches and print the desired information
	for _, match := range matches {
		url := match[1]
		if strings.Contains(url, "/?do=getNewComment") {
			url = strings.ReplaceAll(url, "?do=getNewComment", "")
		}
		title := match[2]
		threads[title] = url
	}
	return threads
}

func GetWebPageBody(url string) string {
	// Create a custom HTTP client with disabled certificate verification
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return ""
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return ""
	}
	return string(body)
}

// GetJsonFromThread Store posts from thread into json format
func GetJsonFromThread(body string) PageJson {
	pattern := `<script[^>]*>([^<]*)<\/script>`

	// Create a regular expression object
	re := regexp.MustCompile(pattern)

	// Find all matches of the pattern in the input string
	matches := re.FindAllStringSubmatch(body, -1)
	//var comments []Comment
	// Iterate over the matches and print each JSON string
	for _, match := range matches {
		if len(match) > 1 {
			jsonString := match[1]
			if strings.Contains(jsonString, "pageStart") {
				//fmt.Println(strings.TrimSpace(jsonString))
				var data PageJson

				// Unmarshal the JSON string into the struct
				err := json.Unmarshal([]byte(jsonString), &data)
				if err != nil {
					fmt.Println("Error:", err)
					return PageJson{}
				}
				return data
			}
		}
	}
	return PageJson{}

}

func GetPageNumberFromThread(json PageJson) int {
	pattern := `page/(\d+)`

	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Find the first match
	match := re.FindStringSubmatch(json.Comments[0].ID)

	if len(match) > 1 {
		// Extract the captured integer value
		pageNumber := match[1]
		page, err := strconv.Atoi(pageNumber)
		if err != nil {
			fmt.Println("Error:", err)
			return 0
		}
		return page
	} else {
		//fmt.Println("No match found.")
		return 0
	}
}

func SendToTgPostWithReplyKeyboard(bot *tgbotapi.BotAPI, id int64, message string, keyboard tgbotapi.ReplyKeyboardMarkup) {
	msg := tgbotapi.NewMessage(id, message)
	msg.ReplyMarkup = keyboard
	_, err := bot.Send(msg)
	if err != nil {
		fmt.Println("Error sending message:", err)
	}

}

func SendToTg(bot *tgbotapi.BotAPI, id int64, message string) {
	msg := tgbotapi.NewMessage(id, message)
	_, err := bot.Send(msg)
	if err != nil {
		fmt.Println("Error sending message:", err)
	}

}

func GetUrlAndTypeOfPreviousPage(url string) (string, string) {

	// Check if the URL matches the subforum pattern
	if subForumRe.MatchString(url) {
		// Check if the URL already contains the "page" part
		if pageRe.MatchString(url) {
			// Increment the page number by 1
			nextURL := pageRe.ReplaceAllStringFunc(url, func(match string) string {
				pageNumberStr := pageRe.FindStringSubmatch(match)[1]
				pageNumber, err := strconv.Atoi(pageNumberStr)
				if err != nil {
					return match
				}
				var decrementedPageNumber int
				if pageNumber > 2 {
					decrementedPageNumber = pageNumber - 1
				} else {
					decrementedPageNumber = 0
				}
				return fmt.Sprintf("page/%d", decrementedPageNumber)
			})
			return nextURL, "SubForum"
		}

		// Append the "page/2" part to the subforum URL
		nextURL := subForumRe.ReplaceAllString(url, "forum/$1/")
		return nextURL, "SubForum"
	}

	// Check if the URL matches the topic pattern
	if topicRe.MatchString(url) {
		// Check if the URL already contains the "page" part
		if pageRe.MatchString(url) {
			// Increment the page number by 1
			nextURL := pageRe.ReplaceAllStringFunc(url, func(match string) string {
				pageNumberStr := pageRe.FindStringSubmatch(match)[1]
				pageNumber, err := strconv.Atoi(pageNumberStr)
				if err != nil {
					return match
				}
				var decrementedPageNumber int
				if pageNumber > 2 {
					decrementedPageNumber = pageNumber - 1
				} else {
					decrementedPageNumber = 0
				}
				//incrementedPageNumber := pageNumber - 1
				return fmt.Sprintf("page/%d", decrementedPageNumber)
			})
			return nextURL, "Thread"
		}

		// Append the "page/2" part to the topic URL
		nextURL := topicRe.ReplaceAllString(url, "topic/$1/")
		return nextURL, "Thread"
	}

	// No match found, return the original URL
	return url, ""
}

func GetUrlAndTypeOfNextPage(url string) (string, string) {

	// Check if the URL matches the subforum pattern
	if subForumRe.MatchString(url) {
		// Check if the URL already contains the "page" part
		if pageRe.MatchString(url) {
			// Increment the page number by 1
			nextURL := pageRe.ReplaceAllStringFunc(url, func(match string) string {
				pageNumberStr := pageRe.FindStringSubmatch(match)[1]
				pageNumber, err := strconv.Atoi(pageNumberStr)
				if err != nil {
					return match
				}
				incrementedPageNumber := pageNumber + 1
				return fmt.Sprintf("page/%d", incrementedPageNumber)
			})
			return nextURL, "SubForum"
		}

		// Append the "page/2" part to the subforum URL
		nextURL := subForumRe.ReplaceAllString(url, "forum/$1/page/2")
		return nextURL, "SubForum"
	}

	// Check if the URL matches the topic pattern
	if topicRe.MatchString(url) {
		// Check if the URL already contains the "page" part
		if pageRe.MatchString(url) {
			// Increment the page number by 1
			nextURL := pageRe.ReplaceAllStringFunc(url, func(match string) string {
				pageNumberStr := pageRe.FindStringSubmatch(match)[1]
				pageNumber, err := strconv.Atoi(pageNumberStr)
				if err != nil {
					return match
				}
				incrementedPageNumber := pageNumber + 1
				return fmt.Sprintf("page/%d", incrementedPageNumber)
			})
			return nextURL, "Thread"
		}

		// Append the "page/2" part to the topic URL
		nextURL := topicRe.ReplaceAllString(url, "topic/$1/page/2")
		return nextURL, "Thread"
	}

	// No match found, return the original URL
	return url, ""
}

func SetPageNumberInURL(url string, pageNum string) (string, string) {

	// Check if the URL matches the subforum pattern
	if subForumRe.MatchString(url) {
		// Check if the URL already contains the "page" part
		if pageRe.MatchString(url) {
			// Replace the existing page number in the URL with the given pageNum
			nextURL := pageRe.ReplaceAllStringFunc(url, func(match string) string {
				return fmt.Sprintf("page/%s", pageNum)
			})
			return nextURL, "SubForum"
		}

		// Append the "page" part to the subforum URL with the given pageNum
		nextURL := url + "page/" + pageNum + "/"
		return nextURL, "SubForum"
	}

	// Check if the URL matches the topic pattern
	if topicRe.MatchString(url) {
		// Check if the URL already contains the "page" part
		if pageRe.MatchString(url) {
			// Replace the existing page number in the URL with the given pageNum
			nextURL := pageRe.ReplaceAllStringFunc(url, func(match string) string {
				return fmt.Sprintf("page/%s", pageNum)
			})
			return nextURL, "Thread"
		}

		// Append the "page" part to the topic URL with the given pageNum
		nextURL := url + "page/" + pageNum + "/"
		return nextURL, "Thread"
	}

	// No match found, return the original URL
	return url, ""
}

// Check if url is valid
func isValidURL(url string) bool {
	// Use a regular expression pattern to validate the URL format
	pattern := `^https?://.+`
	match, _ := regexp.MatchString(pattern, url)
	return match
}

func HandleMainPage(bot *tgbotapi.BotAPI, update tgbotapi.Update, url string) {
	body := GetWebPageBody(url)
	subForums := GetSubForumsFromMainPage(body)
	replyKeyboard := CreateKeyboardFromMapForMainPage(subForums)
	SendToTgPostWithReplyKeyboard(bot, update.Message.Chat.ID, "Список", replyKeyboard)
}

func HandleSubForum(bot *tgbotapi.BotAPI, update tgbotapi.Update, url string) {
	body := GetWebPageBody(url)
	subForums := GetSubForumsFromMainPage(body)
	threads := GetThreadsFromSubForums(body)
	lastCommandFromUser[update.Message.Chat.ID] = url
	// Create keyboard with lists of subforums and threads
	replyKeyboard := CreateKeyboardFromMap(subForums, threads)
	SendToTgPostWithReplyKeyboard(bot, update.Message.Chat.ID, "Список", replyKeyboard)
}

func HandleThread(bot *tgbotapi.BotAPI, update tgbotapi.Update, url string) {
	//Create keyboard with navigation buttons
	mainPageButton := tgbotapi.NewKeyboardButton("Главная")
	prevButton := tgbotapi.NewKeyboardButton("Previous")
	setButton := tgbotapi.NewKeyboardButton("Set page")
	nextButton := tgbotapi.NewKeyboardButton("Next")
	firstRow := tgbotapi.NewKeyboardButtonRow(mainPageButton)
	secondRow := tgbotapi.NewKeyboardButtonRow(prevButton, setButton, nextButton)
	replyKeyboard := tgbotapi.NewReplyKeyboard(firstRow, secondRow)
	replyKeyboard.OneTimeKeyboard = false
	replyKeyboard.ResizeKeyboard = true
	lastCommandFromUser[update.Message.Chat.ID] = url

	body := GetWebPageBody(url)
	jsonData := GetJsonFromThread(body)
	pageNumber := GetPageNumberFromThread(jsonData)
	reply := fmt.Sprintf("Тема \"%s\", страница %d из %d\n", jsonData.Name, pageNumber, jsonData.PageEnd)
	SendToTg(bot, update.Message.Chat.ID, reply)
	// Send posts from thread to user
	for i := range jsonData.Comments {
		if len(jsonData.Comments[i].Text) > 0 {
			post := fmt.Sprintf("%s написал:\n\n%s\n", jsonData.Comments[i].Author.Name, PrettyPrintString(strings.TrimSpace(jsonData.Comments[i].Text)))
			SendToTgPostWithReplyKeyboard(bot, update.Message.Chat.ID, post, replyKeyboard)
		}
	}
}

// CreateKeyboardFromMap Create keyboard with lists of subforums and threads
func CreateKeyboardFromMap(maps ...map[string]string) tgbotapi.ReplyKeyboardMarkup {

	mainPageButton := tgbotapi.NewKeyboardButton("Главная")
	prevButton := tgbotapi.NewKeyboardButton("Previous")
	setButton := tgbotapi.NewKeyboardButton("Set page")
	nextButton := tgbotapi.NewKeyboardButton("Next")
	firstRow := tgbotapi.NewKeyboardButtonRow(mainPageButton)
	secondRow := tgbotapi.NewKeyboardButtonRow(prevButton, setButton, nextButton)
	replyKeyboard := tgbotapi.NewReplyKeyboard(firstRow, secondRow)
	replyKeyboard.OneTimeKeyboard = false
	replyKeyboard.ResizeKeyboard = true
	for _, m := range maps {
		for command, link := range m {
			AddLinkToMap(command, link)
			button := tgbotapi.NewKeyboardButton(command)
			replyKeyboard.Keyboard = append(replyKeyboard.Keyboard, []tgbotapi.KeyboardButton{button})
		}
	}

	return replyKeyboard
}
func CreateKeyboardFromMapForMainPage(maps ...map[string]string) tgbotapi.ReplyKeyboardMarkup {
	replyKeyboard := tgbotapi.NewReplyKeyboard()
	replyKeyboard.OneTimeKeyboard = false
	replyKeyboard.ResizeKeyboard = true
	for _, m := range maps {
		for command, link := range m {
			AddLinkToMap(command, link)
			button := tgbotapi.NewKeyboardButton(command)
			replyKeyboard.Keyboard = append(replyKeyboard.Keyboard, []tgbotapi.KeyboardButton{button})
		}
	}

	return replyKeyboard
}

func main() {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	bot, err := tgbotapi.NewBotAPIWithClient("token", httpClient)
	if err != nil {
		fmt.Println("Error initializing Telegram bot:", err)
		os.Exit(1)
	}

	// Set up updates channel to receive messages
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates, err := bot.GetUpdatesChan(updateConfig)
	if err != nil {
		fmt.Println("Error setting up updates channel:", err)
		return
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		url := strings.TrimSpace(update.Message.Text)
		var linkType string

		if strings.Contains(url, "https:") {
			linkType = GetLinkType(url)
		} else if waitForPageFromUser[update.Message.Chat.ID] == true {
			if num, err := strconv.ParseInt(url, 10, 64); err == nil {
				if num < 0 {
					url = "0"
				}
				url, linkType = SetPageNumberInURL(lastCommandFromUser[update.Message.Chat.ID], url)
				waitForPageFromUser[update.Message.Chat.ID] = false
			} else {
				SendToTg(bot, update.Message.Chat.ID, "Введите корректный номер страницы")
			}
		} else {
			switch url {
			case "Set page":
				SendToTg(bot, update.Message.Chat.ID, "Введите номер страницы")
				waitForPageFromUser[update.Message.Chat.ID] = true
			case "Next":
				url, linkType = GetUrlAndTypeOfNextPage(lastCommandFromUser[update.Message.Chat.ID])
			case "Previous":
				url, linkType = GetUrlAndTypeOfPreviousPage(lastCommandFromUser[update.Message.Chat.ID])
			case "Главная", "/start", "/Start":
				linkType = "MainPage"
				url = "https://prodota.ru/forum/"
			default:
				linkType = linksMap[url].Type
				url = linksMap[url].Url
			}
		}

		if isValidURL(url) {
			switch linkType {
			case "MainPage":
				HandleMainPage(bot, update, url)
			case "SubForum":
				HandleSubForum(bot, update, url)
			case "Thread":
				HandleThread(bot, update, url)
			}
		} else {
			if waitForPageFromUser[update.Message.Chat.ID] == false {
				SendToTg(bot, update.Message.Chat.ID, "Команда неправильная")
			}
		}
	}
}
