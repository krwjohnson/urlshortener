# URL Shortener in Go and MongoDB

A simple URL shortener written in Go, using MongoDB as the database.

## Requirements

- Go
- MongoDB

## Installation & Running

1. Clone this repository:

    ```
    git clone https://github.com/krwjohnson/url-shortener.git
    ```

2. Navigate to the project directory:

    ```
    cd url-shortener
    ```

3. Build and run the project:

    ```
    go build
    go run main.go
    ```

4. Open your browser and visit `http://localhost:8080`.

## Usage

1. Enter a URL into the text field.
2. Click the "Create short URL" button.
3. The short URL will appear below the button. Copy and use it anywhere!

## Future Plans

This is a minimum viable product. Features planned for future include:

- User registration and authentication.
- User dashboard to manage their URLs.
- Analytics to track how many times each short URL is used.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT](LICENSE)
