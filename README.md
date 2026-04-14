# Flo

A cute Go library for making Fluxer bots/self-bots designed to be simple in implementation and usage!
More user-API specific things may be considered in the future, but that is likely more useful for custom clients which is not the current focus.

Join our [Fluxer Community](https://fluxer.gg/bhvnuLCK) to get help or just hang out!

## Features
- Rate limiting
- Caching
- Sharding (even though Fluxer doesn't actually support it yet!)
- Cache and REST methods on objects for slightly more type safety
  - Another approach is typed IDs, but this has its downsides - at least taking the approach of [arikawa](https://github.com/diamondburned/arikawa/blob/v3/discord/snowflake_types.go) which duplicates code and uses codegen.

## See also
- [FluxerGo](https://github.com/fluxergo/fluxergo) - Port of [DisGo](https://github.com/disgoorg/disgo) to Fluxer (more modular)

## Example

```go
token := os.Getenv("FLUXER_TOKEN")
if token == "" {
    slog.Error("please provide the token as FLUXER_TOKEN")
    os.Exit(1)
}
// you need this prefix for bot tokens!
token = "Bot " + token

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

gateway.ShardReady.OnceSync(func(r flo.ReadyEvent) {
    fmt.Println("ready as " + r.User.Tag())
})

gateway.MessageCreate.On(func(m flo.MessageCreateEvent) {
    var resp string
    if m.Content == "!ping" {
        resp = "Ping!"
    } else if m.Content == "!pong" {
        resp = "Pong!"
    } else {
        return
    }

    _, err := rest.CreateMessage(context.TODO(), m.ChannelID, flo.CreateMessageOpts{
        Content: resp,
        // reply to the original message
        MessageReference: flo.MessageReferenceOpts{
            MessageID: m.ID,
        },
    })
    if err != nil {
        slog.Warn("couldn't reply to command :/", slog.Any("err", err))
    }
})

gateway.Connect()

stopped, _ := gateway.ShardStopped.OnceChan()
event := <-stopped
if event.Err != nil {
    slog.Error("stopped with error", slog.Any("err", event.Err))
}
```
