# 🛒 Telegram Market Deals Bot

This Go project scrapes promotional flyers (PDFs and images) from multiple supermarket websites, converts them into images, and sends them to a Telegram channel automatically.

## ✨ Features

- ✅ Automatically scrapes weekly promotional flyers from:
  - **Roldão Atacadista**
  - **Supermercados Pague Menos**
  - **Delta Supermercados**
  - **Supermercados São Vicente**
- ✅ Detects whether an offer is new by hashing the PDF/image and checking it in a PostgreSQL database.
- ✅ Converts PDFs into PNG images using `pdftoppm`.
- ✅ Groups and sends up to 10 images per message to a Telegram channel.
- ✅ Sends a summary report to the admin on Telegram.

## 📦 Folder Structure

```
tmp/
├── <Market>/
│   └── <timestamp>/
│       ├── page-1.png
│       ├── page-2.png
...
```

This temporary structure is used to store images before sending to Telegram.

## 🛠️ Requirements

- Go 1.20+
- PostgreSQL
- Telegram Bot Token
- `pdftoppm` installed (`poppler-utils` package)
- Environment variables:
  - `DATABASE_URL` – connection string for your PostgreSQL database
  - `TELEGRAM_BOT_TOKEN` – your Telegram Bot API token
  - `CHANNEL_ID` – the Telegram channel ID to post offers to
  - `ADMIN_CHAT_ID` – the Telegram user ID to send weekly report to

## 📥 Installation

1. Clone the repo:

```bash
git clone https://github.com/yourusername/telegram-market-bot.git
cd telegram-market-bot
```

2. Install Go dependencies:

```bash
go mod tidy
```

3. Set up your `.env` or export environment variables:

```bash
export DATABASE_URL=postgres://user:pass@localhost:5432/dbname
export TELEGRAM_BOT_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
export CHANNEL_ID=-1001234567890
export ADMIN_CHAT_ID=123456789
```

4. Run the bot:

```bash
go run main.go
```

## 🧠 How It Works

1. **Scraping:**  
   Each `scrap<Supermarket>()` function fetches the current offer link from the store’s website.

2. **Download & Hashing:**  
   Flyers are downloaded and hashed (SHA-256). If the hash is not in the database, the file is saved and stored.

3. **PDF to Image:**  
   `pdftoppm` is used to convert PDFs into PNGs.

4. **Sending to Telegram:**  
   Images are sent in batches (max 10 per group) to a Telegram channel using the Bot API.

5. **Weekly Report:**  
   A report is sent to the admin summarizing which markets had offers that week.

## 🧪 Database

You must generate the SQL boilerplate using [sqlc](https://sqlc.dev/), based on your PostgreSQL schema.

The `tabloides` table should contain at least:

```sql
CREATE TABLE tabloides (
  id CHAR(64) PRIMARY KEY,
  mercado VARCHAR(50) NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  protected BOOLEAN DEFAULT FALSE
);
```

## 🔄 Cronjob / Automation

You can set this to run automatically every day or week using a cronjob:

```bash
0 9 * * * /usr/local/go/bin/go run /path/to/main.go >> /var/log/market-bot.log 2>&1
```

## ⚠️ Notes

- Make sure your bot has permission to post in the channel.
- If you're running behind a proxy, adjust the HTTP client as needed.
- This project disables TLS verification (`InsecureSkipVerify: true`) to bypass broken HTTPS implementations — **use with caution.**