package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"octavio/mercado-telegram/db"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"
)

func main() {
	log.Println("Starting bot")
	err := os.MkdirAll("tmp", 0755)

	if err != nil {
		log.Fatalf("Error creating directory: %v", err)
	}

	ctx := context.Background()

	conn, err := connectWithRetry(ctx, os.Getenv("DATABASE_URL"), 10)

	if err != nil {
		fmt.Println("ERROR DB")
	}
	defer conn.Close(ctx)

	queries := db.New(conn)

	if err != nil {
		log.Fatalf("Error creating directory: %v", err)
	}
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}
	log.Println("Extraindo ofertas do Roldão Atacadista")
	scrapRoldao(client, queries)
	log.Println("Enviando ofertas do Roldão Atacadista")
	sendMessages(bot)
	log.Println("Extraindo ofertas do Supermercados Pague Menos")
	scrapPagueMenos(client, queries)
	log.Println("Enviando ofertas do Supermercados Pague Menos")
	sendMessages(bot)
	log.Println("Extraindo ofertas do Delta Atacadista")
	scrapDelta(client, queries)
	log.Println("Enviando ofertas do Delta Atacadista")
	sendMessages(bot)
	log.Println("Extraindo ofertas do Supermercados São Vicente")
	scrapSaoVicente(client, queries)
	log.Println("Enviando ofertas do Supermercados São Vicente")
	sendMessages(bot)
	queries.DeleteOld(context.Background())
	reports(bot, queries)
	log.Println("Enviando relatório semanal")
}

func connectWithRetry(ctx context.Context, dbURL string, maxRetries int) (*pgx.Conn, error) {
	var conn *pgx.Conn
	var err error

	for i := 0; i < maxRetries; i++ {
		conn, err = pgx.Connect(ctx, dbURL)
		if err == nil {
			log.Println("✅ Conectado ao banco com sucesso!")
			return conn, nil
		}

		wait := time.Duration(15<<i) * time.Second // Ex: 2s, 4s, 8s, 16s...
		log.Printf("⏳ Tentativa %d: falha ao conectar ao banco. Esperando %s antes de tentar novamente...\n", i+1, wait)
		time.Sleep(wait)
	}

	return nil, fmt.Errorf("❌ Falha ao conectar ao banco após %d tentativas: %w", maxRetries, err)
}

func reports(bot *tgbotapi.BotAPI, queries *db.Queries) {
	tabloides, err := queries.GetLastWeek(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	counter := make(map[string]int)

	counter["Roldão Atacadista"] = 0
	counter["Supermercados Pague Menos"] = 0
	counter["Delta Supermercados"] = 0
	counter["Supermercados São Vicente"] = 0

	for _, tabloide := range tabloides {
		counter[tabloide.Mercado]++
	}

	toSend := ""

	for mercado, count := range counter {
		status := ""
		if count > 0 {
			status = "✅"
		} else {
			status = "❌"
		}
		toSend += fmt.Sprintf("%s: %s\n", mercado, status)
	}

	ADMIN_CHAT_ID, err := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
	if err != nil {
		panic("CHANNEL_ID must be an integer")
	}

	bot.Send(tgbotapi.NewMessage(ADMIN_CHAT_ID, "Relatório de ofertas da semana:\n\n"+toSend))
}

func pdfToImages(mercado string) {
	toCreate := filepath.Join("tmp", mercado, fmt.Sprintf("%d", time.Now().Unix()))
	os.MkdirAll(toCreate, 0755)
	cmd := exec.Command("pdftoppm", "-r", "80", filepath.Join("tmp", mercado, "ofertas.pdf"), filepath.Join(toCreate, "page"), "-png")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func sendMessages(bot *tgbotapi.BotAPI) {
	CHANNEL_ID, err := strconv.ParseInt(os.Getenv("CHANNEL_ID"), 10, 64)
	if err != nil {
		panic("CHANNEL_ID must be an integer")
	}
	files, err := os.ReadDir("tmp")
	if err != nil {
		panic(err)
	}

	var sendPool []tgbotapi.Chattable

	var mediaGroup []any

	for _, file := range files {
		if file.IsDir() {
			timestamps, _ := os.ReadDir(filepath.Join("tmp", file.Name()))
			if len(timestamps) > 0 {
				sendPool = append(sendPool, tgbotapi.NewMessage(CHANNEL_ID, fmt.Sprintf("Ofertas do %s", file.Name())))
				sort.Slice(timestamps, func(i, j int) bool {
					first, _ := strconv.Atoi(timestamps[i].Name())
					last, _ := strconv.Atoi(timestamps[j].Name())
					return first < last
				})
				for _, timestamp := range timestamps {
					if timestamp.IsDir() {
						imgs, _ := os.ReadDir(filepath.Join("tmp", file.Name(), timestamp.Name()))
						count := 0
						for _, img := range imgs {
							count++
							if !img.IsDir() && (strings.HasSuffix(img.Name(), ".png") || strings.HasSuffix(img.Name(), ".jpg")) {
								msg := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(filepath.Join("tmp", file.Name(), timestamp.Name(), img.Name())))
								mediaGroup = append(mediaGroup, msg)
								if count%10 == 0 || count == len(imgs) {
									sendPool = append(sendPool, tgbotapi.NewMediaGroup(CHANNEL_ID, mediaGroup))
									mediaGroup = nil
								}
							}
						}
					}
				}
			}
		}
	}
	for _, msg := range sendPool {
		bot.Send(msg)
	}

	os.RemoveAll("tmp")
	os.MkdirAll("tmp", 0755)
}

func scrapRoldao(client *http.Client, queries *db.Queries) {
	resp, err := client.Get("https://roldao.com.br/ofertas-do-roldao")
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(".post-item.isotope-item.clearfix.post.type-post.status-publish.format-standard.has-post-thumbnail.hentry.category-ofertas.tag-diar.tag-ofertas.tag-pagar-barato.tag-pepsico.tag-preco-baixo.tag-quero-ofertas.tag-roldao.tag-seu-negocio.tag-sua-casa").Each(func(i int, s *goquery.Selection) {
		selector, exists := s.Find("a").Attr("href")
		if exists {
			resp, err := client.Get(selector)
			if err != nil {
				fmt.Println("Error making request:", err)
				return
			}
			defer resp.Body.Close()
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				log.Fatal(err)
			}

			link, err := doc.Find("#real3d_flipbook_embed-js-extra").First().Html()

			if err != nil {
				fmt.Println("Error extracting")
			}

			link = html.UnescapeString(link)
			link, err = url.QueryUnescape(link)

			if err != nil {
				fmt.Println("Error unescaping URL:", err)
				return
			}
			link = strings.Replace(link, "/* <![CDATA[ */", "", 1)
			link = strings.Replace(link, "/* ]]> */", "", 1)
			link = strings.Split(link, " = \"")[1]
			link = strings.TrimSpace(link)
			link = link[:len(link)-2]

			link = strings.ReplaceAll(link, "\\", "")

			var data map[string]interface{}

			err = json.Unmarshal([]byte(link), &data)
			if err != nil {
				panic(err)
			}

			url := data["pdfUrl"].(string)
			existsDb, err := downloadFile("ofertas.pdf", url, client, queries, "Roldão Atacadista")
			if err != nil {
				fmt.Println("Error downloading file:", err)
				return
			}
			if !existsDb {
				pdfToImages("Roldão Atacadista")
			}
		}
	})
}

func scrapPagueMenos(client *http.Client, queries *db.Queries) {
	resp, err := client.Get("https://www.superpaguemenos.com.br/jornal-de-ofertas")
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var links []*goquery.Selection

	doc.Find(".showcase-shelf-banner .text-center a").Each(func(i int, s *goquery.Selection) {
		links = append(links, s)
	})
	doc.Find(".owl-stage .item a").Each(func(i int, s *goquery.Selection) {
		links = append(links, s)
	})

	for _, s := range links {
		href, exists := s.Attr("href")

		if exists {
			if !strings.Contains(href, "superpaguemenos.com.br") && !strings.Contains(href, "//io.convertiez.com.br") {
				href = "https://www.superpaguemenos.com.br" + href
			} else if strings.Contains(href, "//io.convertiez.com.br") {
				href = "https://" + href[2:]
			}
			existsDb, err := downloadFile("ofertas.pdf", href, client, queries, "Supermercados Pague Menos")
			if err != nil {
				log.Fatal(err)
				return
			}
			if !existsDb {
				pdfToImages("Supermercados Pague Menos")
			}
		}
	}
}

func scrapDelta(client *http.Client, queries *db.Queries) {
	resp, err := client.Get("https://www.deltasuper.com.br/ofertas-salto/")
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err == nil {
		doc.Find(".jet-listing-grid__item").Each(func(i int, s *goquery.Selection) {
			timestamp := time.Now().Unix()
			os.MkdirAll(filepath.Join("tmp", "Delta Supermercados", strconv.Itoa(int(timestamp))), 0644)
			href, exists := s.Find("a").First().Attr("href")
			if exists {
				resp, err := client.Get(href)
				if err != nil {
					fmt.Println("Error making request:", err)
					return
				}
				defer resp.Body.Close()
				doc, err := goquery.NewDocumentFromReader(resp.Body)
				if err == nil {
					doc.Find(".gallery-item").Each(func(j int, s *goquery.Selection) {
						href, exists := s.Find("a").First().Attr("href")
						if exists {
							downloadFile(filepath.Join(strconv.Itoa(int(timestamp)), fmt.Sprintf("%d-%d.jpg", i, j)), href, client, queries, "Delta Supermercados")
						}
					})
				}
			}
		})
	}
}

func scrapSaoVicente(client *http.Client, queries *db.Queries) {
	resp, err := client.Get("https://www.svicente.com.br/ofertas")
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err == nil {
		doc.Find("#Salto div.experience-saoVicente_layouts-gridItem div.viewFlyer_component div.img_desktop").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Find("a[target=_blank]").First().Attr("href")
			if exists {
				existsDb, err := downloadFile("ofertas.pdf", "https://www.svicente.com.br"+href, client, queries, "Supermercados São Vicente")
				if err != nil {
					log.Fatal(err)
					return
				}
				if !existsDb {
					pdfToImages("Supermercados São Vicente")
				}
			}
		})
	}
}

func downloadFile(filename string, url string, client *http.Client, queries *db.Queries, mercado string) (bool, error) {
	pathMercado := filepath.Join("tmp", mercado)
	err := os.MkdirAll(pathMercado, 0755)
	if err != nil {
		return false, err
	}
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	h := sha256.New()
	tee := io.TeeReader(resp.Body, h)

	// Precisamos primeiro ler todo o corpo para calcular o hash corretamente.
	// Para isso, salvamos os dados em um buffer temporário.
	var buf bytes.Buffer
	_, err = io.Copy(&buf, tee)
	if err != nil {
		return false, err
	}

	// Agora o hash está calculado corretamente
	sha256sum := fmt.Sprintf("%x", h.Sum(nil))

	// Verifica no banco
	returned, _ := queries.GebById(context.Background(), sha256sum)
	if returned.ID == "" {
		// Salva o conteúdo no arquivo
		err = os.WriteFile(filepath.Join(pathMercado, filename), buf.Bytes(), 0644)
		if err != nil {
			return false, err
		}

		queries.CreateTabloide(context.Background(), db.CreateTabloideParams{
			ID:      sha256sum,
			Mercado: mercado,
		})
		return false, nil
	} else {
		return true, nil
	}
}
