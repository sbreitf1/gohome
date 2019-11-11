# GoHome

Util to show current worktime and possible leave times with Dorma integration.

![example view of current worktime](example.png)

## Getting started

To download and install the utility execute the following commands:

```
go get github.com/sbreitf1/gohome
go install github.com/sbreitf1/gohome
```

Now start it with `gohome` from command line.

You will probably be asked to enter a Dorma host. Paste the same host here you visit in your browser (including protocol and path). Finally, you need to enter your Dorma credentials.

![blub](login.png)

These values are remembered in `~/.dorma/` and are used in all following runs. It's plaintext, but you can encrypt the file `~/.dorma/host-credentials` with a PGP key of your choice (must add `.pgp` to filename) and supply `-k <Key-ID>` to subsequent gohome-calls.

## Thanks

Thanks to `danielb42` for the [initial idea and cool project name](https://github.com/danielb42/gohome)!