package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/chromedp/cdproto/dom"

	"github.com/chromedp/cdproto/cdp"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

//DefaulUserAgent agent default
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.61 Safari/537.36"

//Product product struct
type Product struct {
	Name        string
	Description string
	ImageLink   string
	Price       string
	Rate        string
	StoreName   string
	Index       int
}

type Job struct {
	Link  string
	Index int
}

func getProductDetail(link string, index int) Product {
	ctx, cancel := chromedp.NewContext(context.Background())
	fmt.Printf("Fetching product index : %d \n", index)
	defer cancel()
	product := Product{
		Index: index,
	}
	var ok bool
	err := chromedp.Run(ctx,
		emulation.SetUserAgentOverride(DefaultUserAgent),
		chromedp.Navigate(link),
		chromedp.WaitVisible(`[data-testid=lblPDPDetailProductName]`),
		chromedp.Text(`[data-testid=lblPDPDetailProductName]`, &product.Name),
		chromedp.Text(`[data-testid=lblPDPDescriptionProduk]`, &product.Description),
		chromedp.Text(`[data-testid=lblPDPDetailProductPrice]`, &product.Price),
		chromedp.Text(`[data-testid=llbPDPFooterShopName]>h2`, &product.StoreName),
		chromedp.Text(`[data-testid=lblPDPDetailProductRatingNumber]`, &product.Rate),
		chromedp.AttributeValue(`[data-testid=PDPMainImage]`, "src", &product.ImageLink, &ok),
	)
	if err != nil {
		panic(err)
	}

	return product
}

func worker(id int, jobs <-chan Job, results chan<- Product) {
	for j := range jobs {
		fmt.Println("worker", id, "fetching  index", j.Index)
		product := getProductDetail(j.Link, j.Index)
		fmt.Println("worker", id, "fetched", j.Index)
		results <- product
	}
}

func main() {
	const productNumber = 100
	const numberWorker = 5

	jobs := make(chan Job, productNumber)
	results := make(chan Product, productNumber)

	for w := 1; w <= numberWorker; w++ {
		go worker(w, jobs, results)
	}

	productLinks := fetchProductLink(productNumber)

	//Doing Job For each link
	for j := 0; j < productNumber; j++ {
		jobs <- Job{
			Index: j,
			Link:  productLinks[j],
		}
	}
	close(jobs)

	products := make([]Product, productNumber)

	for i := 0; i < productNumber; i++ {
		var product = <-results
		products[product.Index] = product
	}

	//csv
	writeCsv(products, "products.csv")

	defer close(results)
}

func fetchProductLink(n int) []string {
	fmt.Println("Fetching Product List...")

	products := make([]string, 0)
	page := 1

	for ; len(products) < n; page++ {
		ctx, cancel := chromedp.NewContext(context.Background())

		defer cancel()

		err := chromedp.Run(ctx,
			emulation.SetUserAgentOverride(DefaultUserAgent),
			chromedp.Navigate(fmt.Sprintf("https://www.tokopedia.com/p/handphone-tablet/handphone?page=%d", page)),
			chromedp.ScrollIntoView(".IOLazyloading"),
			chromedp.WaitNotPresent(".IOLazyloading"),
		)
		if err != nil {
			panic(err)
		}

		var ids []cdp.NodeID
		var nodes []*cdp.Node
		if err := chromedp.Run(ctx,
			chromedp.NodeIDs(`[data-testid=lstCL2ProductList]`, &ids),
			chromedp.ActionFunc(func(c context.Context) error {
				return dom.RequestChildNodes(ids[0]).WithDepth(2).Do(c)
			}),
			chromedp.Nodes(`[data-testid=lstCL2ProductList]>div`, &nodes, chromedp.ByQueryAll),
		); err != nil {
			panic(err)
		}

		for i, node := range nodes {
			if i%35 >= 5 { // skip promo for every first 5 item on 35 item
				link := node.Children[0].AttributeValue("href")

				products = append(products, link)

				if len(products) == n {
					break
				}
			}
		}
	}

	return products
}

func writeCsv(products []Product, file string) {
	datas := [][]string{
		{"Name", "Description", "Image Link", "Price", "Rating", "Name of store or merchat"},
	}

	for _, product := range products {
		datas = append(datas, []string{
			product.Name,
			product.Description,
			product.ImageLink,
			product.Price,
			product.Rate,
			product.StoreName,
		})
	}

	csvFile, err := os.Create(file)
	if err != nil {
		panic(fmt.Sprintf("failed creating file: %s", err))
	}

	csvwriter := csv.NewWriter(csvFile)

	for _, row := range datas {
		_ = csvwriter.Write(row)
	}

	csvwriter.Flush()
	csvFile.Close()
}
