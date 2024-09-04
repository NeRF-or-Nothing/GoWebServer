This is the portion of the VidToNerf backend responsible for HTTP communication, worker job delegation, and database management.

# Getting Started
We are always looking for help to improve NeRF-or-Nothing!

As this repository is only a single service in the greater 
system architecture, consider visiting the [Complete Backend](https://github.com/NeRF-or-Nothing/vidtonerf) and [Frontend](https://github.com/NeRF-or-Nothing/Web-App-Vite).

Most of our services are expected to run in their own docker container, this included.

If you want to contribute to the code base, we suggest reading the [Wiki](https://github.com/NeRF-or-Nothing/GoWebServer/wiki) First, and then diving deeper into the go doc's and code base.

# Running the Server
Make sure you have the following installed and running on their respective URLs.
- MongoDB
- RabbitMQ

1. Clone the repository:
   ```
   git clone https://github.com/NeRF-or-Nothing/VidGoNerf.git
   cd VidGoNerf
   ```

2. Set up your environment variables:
   Create a `.env` file in the project root and add the following:
   ```
   MONGO_INITDB_ROOT_USERNAME=your_mongodb_username
   MONGO_INITDB_ROOT_PASSWORD=your_mongodb_password
   RABBITMQ_DEFAULT_USER=your_rabbitmq_username
   RABBITMQ_DEFAULT_PASS=your_rabbitmq_password
   JWT_SECRET=your_jwt_secret
   ```

## On a container (Docker):
Make sure you have the following installed:
- Docker

3. Build the and run image
  ```
  docker build -t web-server
  docker run web-server
  ```

## On my machine (No Docker)
Make sure you have the following installed:
- Go (version 1.22 or later)

3. Install dependencies:
   ```
   go mod download
   ```

4. Build the project:
   ```
   go build ./cmd/webserver
   ```

5. Run the server:
   ```
   ./webserver
   ```

# Contributing / Quick-Start
This guide will help you get started with contributing to our project.

1. Join our [Discord](https://discord.gg/6QAc3FgNSc)
2. Find out what you can do to help directly from the team
3. Start attending RCOS

## Project Structure

- `/cmd/webserver`: Main application entry point
- `/internal`: Internal packages
  - `/log`: Logging utilities
  - `/models`: Data models and database managers
  - `/services`: Business logic and services
- `/web`: Web server and HTTP handlers

## Key Components

1. **WebServer**: Handles HTTP requests and routes them to appropriate handlers.
2. **ClientService**: Manages business logic for client requests.
3. **AMPQService**: Handles communication with RabbitMQ for job processing.
4. **SceneManager**: Manages scene data in the database.
5. **UserManager**: Handles user-related operations.
6. **QueueListManager**: Manages processing queues.

## Making Contributions

1. Create a new branch for your feature or bugfix:
   ```
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and commit them:
   ```
   git add .
   git commit -m "Add your commit message here"
   ```

3. Push your changes to your fork:
   ```
   git push origin feature/your-feature-name
   ```

4. Create a pull request on GitHub.

## Testing

Run the tests using:
```
go test ./...
```

## Code Style

We follow the standard Go code style. Please run `gofmt` on your code before submitting a pull request:
```
gofmt -w .
```

## Documentation

- Please update the relevant documentation when making changes.
- Add comments to your code, especially for complex logic.

## Getting Help

If you have any questions or need help, please:
1. Check the existing issues on GitHub.
2. If you can't find an answer, create a new issue with your question.
3. Reach out in our [Discord](https://discord.gg/6QAc3FgNSc)
