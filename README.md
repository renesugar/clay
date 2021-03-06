# clay
Minimal server platform for gRPC and REST+Swagger APIs

Using clay you can automatically spin up HTTP handlers for your gRPC server with
complete Swagger defs with a few lines of code.

## Why?
There's an excellent [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) proxy generator, 
but it requires you to spin up (at least) one proxy instance in addition to your services.
`clay` allows you to serve HTTP traffic by server instances themselves for easier debugging/testing. 

## How?
Check out an [example server](https://github.com/utrack/clay/wiki/Build-and-run-an-example-SummatorService-using-clay-Server)
for a quick start, or dive into [step-by-step docs](https://github.com/utrack/clay/wiki/Creating-your-API-description)
for a full guide.

### Flexibility
`clay.Server` is easily extendable, as you can pass any options gRPC server can use, 
but if it's not extendable enough then you can use the `.GetDescription()` method 
of your implementation to register the service in your own custom server 
(see [ServiceDesc](https://github.com/utrack/clay/blob/master/transport/handlers.go#L17)).
[clay/server vs own server](https://github.com/utrack/clay/wiki/clay.Server-vs-your-own-server) for more info
regarding BYOS.
