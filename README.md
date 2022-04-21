# Golang Tradningview Binance Webhook

Fork from: https://github.com/cryptothedev/tradingview-binance-webhook


## Dependencies

| Github |
| ------ |
| https://github.com/adshao/go-binance/v2 |
| https://github.com/joho/godotenv |
| https://github.com/go-chi/chi/v5 |
| https://github.com/go-chi/render |

## Installation

```sh
git clone https://github.com/Devestor/golang-tradingview-binance-webhook.git
cd golang-tradingview-binance-webhook
go mod tidy
go mod vendor
go run main.go 
```

## Environments
```sh
BINANCE_API_KEY={BINANCE_API_KEY}
BINANCE_API_SECRET={BINANCE_API_SECRET}
LEVERAGE={LEVERAGE}
TAKE_PROFIT_PERCENTAGE={TAKE_PROFIT_PERCENTAGE}
STOP_LOSS_PERCENTAGE={STOP_LOSS_PERCENTAGE}
PORT={PORT}
TOKEN_WHITELIST={TOKEN_WHITELIST}
```

## Docker

```sh
cd golang-tradingview-binance-webhook
docker build -t golang-tradingview-binance-webhook .
```


Once done, run the Docker image and map the port to whatever you wish on
your host. In this example, we simply map port 6464 of the host to
port 6464 of the Docker (or whatever port was exposed in the Dockerfile):

```sh
docker run -it -d --restart=always -v $(pwd)/.env:/.env -p 6464:6464 golang-tradingview-binance-webhook
```


Verify the deployment by navigating to your server address in
your preferred browser.

```sh
127.0.0.1:8000 or localhost:6464
```


## Run Server Testing
install ngrok link: https://ngrok.com/download
```sh
docker run -it -d --restart=always -v $(pwd)/.env:/.env -p 6464:6464 golang-tradingview-binance-webhook
ngrok http 6464
copy Forwarding https:xxxxx.ngrok.io to tradingview webhook
```


## Example
Symbol_Side_Amount_IsTP_IsSL_IsCheckWL

```sh
{{ticker}}_LONG_50_true_false_false
```

