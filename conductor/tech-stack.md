# Tech Stack

## Programming Languages

- **Go:** The primary programming language for the Fractal Engine, leveraging its concurrency features and strong typing for building a robust and performant RWA engine.

## Key Frameworks and Libraries

- **connectrpc.com/connect:** Used for building RPC services, enabling efficient communication between different components of the system.
- **github.com/btcsuite/btcd:** Provides essential Bitcoin-related functionalities, adapted for integration with the Dogecoin blockchain.
- **github.com/golang-migrate/migrate/v4:** Employed for managing database migrations, ensuring schema evolution is controlled and reproducible.
- **github.com/lib/pq:** The PostgreSQL driver, used for interacting with PostgreSQL databases, a common choice for relational data storage.
- **github.com/mattn/go-sqlite3:** The SQLite driver, offering an embedded, file-based database solution suitable for development, testing, and specific deployment scenarios.
- **github.com/urfave/cli/v3:** Utilized for building the command-line interface (CLI) tools associated with the Fractal Engine, facilitating administrative and user interactions.
- **github.com/testcontainers/testcontainers-go:** Used for integration testing, enabling the spinning up of real services (like databases) in Docker containers for reliable and isolated tests.

## Databases

- **PostgreSQL:** A powerful, open-source relational database system, used for persistent data storage, especially in production environments where scalability and data integrity are paramount.
- **SQLite:** A lightweight, serverless, self-contained, high-reliability, full-featured, public-domain SQL database engine, ideal for local development, testing, and simpler deployment needs.
