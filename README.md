# Flo

A cute Go library for making Fluxer bots designed to be not overly abstracted but still pleasant to use!
User-API related things may also be considered in the future.

```go
token := "Bot " + os.Getenv("FLUXER_TOKEN")

cache := flo.NewCacheDefault()
// don't wanna cache something? just set it to nil!

rest := flo.REST{
    Auth:  token,
    Cache: &cache,
}

gateway := flo.Gateway{
    Auth:  token,
    Cache: &cache,
}

gateway.Ready.Once(func(r flo.ReadyEvent) {
    fmt.Println("ready as " + r.User.Tag())
})
gateway.MessageCreate.On(func(m flo.Message) {
    if m.Content == "!ping" {
        rest.SendMessageContent(context.TODO(), m.ChannelID, "pong!")
    }
})

err := gateway.Connect()

s := make(chan os.Signal, 1)
os.Notify(s, syscall.SIGINT, syscall.SIGTERM)

<-s
```
