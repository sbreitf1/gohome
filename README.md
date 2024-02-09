# GoHome

Util to show current worktime and possible leave times with Matrix integration.

![example view of current worktime](example.png)

## Getting started

To download and install the utility execute the following commands:

```
go install github.com/sbreitf1/gohome@v1.4.2
```

Now start it with `gohome` from command line.

You will probably be asked to enter a Matrix host. Paste the same host you visit in your browser (including protocol and path) here. Finally, you need to enter your Matrix credentials.

![Login example](login.png)

These values are stored in `~/.config/gohome` (XDG compatible) and are used in all following runs. You can enter an empty password here to only store host and username. You will be prompted for your password on every run.

Old configs in `~/.gohome` will be automatically migrated.

## Thanks

- Thanks to [danielb42](https://github.com/danielb42) for the [initial idea and cool project name](https://github.com/danielb42/gohome)
- Thanks to [fleaz](https://github.com/fleaz) for suggesting XDG config
- Thanks to [Nerzal](https://github.com/Nerzal) for Matrix v4.2.2 Support
