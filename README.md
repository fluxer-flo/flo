# Flo

A cute Go library for making Fluxer bots designed to be not overly abstracted but still pleasant to use!
User-API related things may also be considered in the future.

```go
token := "Bot " + os.Getenv("FLUXER_TOKEN")

cache := flo.NewCacheDefault()
// if you want to change the limit or avoid caching something entirely:
// cache.Guilds = flo.NewCollection[Guild](0)

// REST is used to perform actions through Fluxer's REST HTTP API
rest := flo.REST{
    Auth:  token,
    Cache: &cache,
}

// Gateway is used to receive events through a persistent websocket connection to Fluxer's gateway
gateway := flo.Gateway{
    Auth:  token,
    Cache: &cache,
}

gateway.Ready.Once(func(r flo.ReadyEvent) {
    fmt.Println("ready as " + r.User.Tag())
})
gateway.MessageCreate.On(func(m flo.Message) {
    if m.Content == "!ping" {
        rest.CreateMessage(context.TODO(), m.ChannelID, flo.CreateMessageOpts{
            Content: "pong!"
        })
    } else if m.Content == "!pong" {
        rest.CreateMessage(context.TODO(), m.ChannelID, flo.CreateMessageOpts{
            Content: "ping!"
        })
    }
})

err := gateway.Connect()

s := make(chan os.Signal, 1)
os.Notify(s, syscall.SIGINT, syscall.SIGTERM)

<-s
```
