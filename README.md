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

These values are remembered in `~/.dorma/` and are used in all following runs.

## Credential Encryption

Use the parameter `--keyname` (or `-k`) to specify a PGP key installed to your local keystore. That key will be used to encrypt your credentials file:

```
gohome -k me@example.com
```

An unencrypted credentials file will be automatically encrypted once you pass the key parameter. To go back to using a raw credentials file you need to manually decrypt the file `~/.dorma/host-credentials.gpg` to the raw file `~/.dorma/host-credentials`.

## Thanks

Thanks to `danielb42` for the [initial idea and cool project name](https://github.com/danielb42/gohome)!