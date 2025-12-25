# CLIProxyAPI Plus

English | [Chinese](README_CN.md)

This is the Plus version of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), adding support for third-party providers on top of the mainline project.

All third-party provider support is maintained by community contributors; CLIProxyAPI does not provide technical support. Please contact the corresponding community maintainer if you need assistance.

The Plus release stays in lockstep with the mainline features.

## Differences from the Mainline

- Added Kiro (AWS CodeWhisperer) support (OAuth login), provided by [fuko2935](https://github.com/fuko2935/CLIProxyAPI/tree/feature/kiro-integration), [Ravens2121](https://github.com/Ravens2121/CLIProxyAPIPlus/)

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the mainline repository.

## License

This project is licensed under the MIT License - see `LICENSE`.

## Disclaimer

This project is intended for educational and interoperability research purposes only. It interacts with various internal or undocumented APIs (such as Google's `v1internal`) to provide compatibility layers.

- **Use at your own risk.** The authors are not responsible for any consequences arising from the use of this software, including but not limited to account suspensions or service interruptions.
- This project is **not** affiliated with, endorsed by, or associated with Google, OpenAI, Anthropic, or any other service provider mentioned.
- Users are expected to comply with the Terms of Service of the respective platforms they connect to.
