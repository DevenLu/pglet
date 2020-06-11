package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pglet/pglet/page"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
)

const (
	apiRoutePrefix      string = "/api"
	contentRootFolder   string = "./client/build"
	siteDefaultDocument string = "index.html"
)

func removeElementAt(source []int, pos int) []int {
	copy(source[pos:], source[pos+1:]) // Shift a[i+1:] left one index.
	source[len(source)-1] = 0          // Erase last element (write zero value).
	return source[:len(source)-1]      // Truncate slice.
}

func createPage() *page.Page {
	p, err := page.New("test page 1")
	if err != nil {
		log.Fatal(err)
	}

	p.AddControl(page.NewControl("Row", "", "0"))
	p.AddControl(page.NewControl("Column", "0", "1"))
	p.AddControl(page.NewControl("Column", "0", "2"))

	ctl3 := page.NewControl("Text", "1", "3")
	p.AddControl(ctl3)

	ctl4 := page.NewControl("Button", "2", "4")
	ctl4["text"] = "Click me!"
	p.AddControl(ctl4)

	ctl5, err := page.NewControlFromJson(`{
		"i": "myBtn",
		"p": "2",
		"t": "Button",
		"text": "Cancel"
	  }`)

	if err != nil {
		log.Fatal(err)
	}

	//fmt.Println(ctl5)

	p.AddControl(ctl5)

	return p
}

func main() {

	//fmt.Printf("string: %s", "sss")

	p := createPage()

	//fmt.Println(ctl3)

	//ctl1 := page.controls["ctl_1"]

	var jsonPage string
	j, err := json.MarshalIndent(&p, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}
	jsonPage = string(j)

	fmt.Printf("----------------\n%+v\n--------------\n", jsonPage)

	_, err1 := page.New("test page 2")
	if err1 != nil {
		log.Fatal(err1)
	}

	fmt.Println(page.Pages())

	p2 := page.Page{}

	err = json.Unmarshal([]byte(jsonPage), &p2)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v\n", p2)

	arr := []int{1, 2, 3, 4, 5, 6}

	arr = removeElementAt(arr, 1)
	fmt.Println(arr)

	return

	// Set the router as the default one shipped with Gin
	router := gin.Default()

	// Serve frontend static files
	router.Use(static.Serve("/", static.LocalFile(contentRootFolder, true)))

	// Setup route group for the API
	api := router.Group(apiRoutePrefix)
	{
		api.GET("/", func(c *gin.Context) {
			time.Sleep(4 * time.Second)
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})
	}

	// unknown API routes - 404, all the rest - index.html
	router.NoRoute(func(c *gin.Context) {
		fmt.Println(c.Request.RequestURI)
		if !strings.HasPrefix(c.Request.RequestURI, apiRoutePrefix+"/") {
			c.File(contentRootFolder + "/" + siteDefaultDocument)
		}
	})

	// Start and run the server
	router.Run(":5000")
}
