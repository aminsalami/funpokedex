### How to run

**Docker (recommended)**
```bash
docker compose up --build
```

**Locally**
```bash
export POKEAPI_BASE_URL=https://pokeapi.co
export TRANSLATIONS_BASE_URL=https://funtranslations.mercxry.me
go run .
```

The API listens on port `8080`.

---

### Note#1

There would be other design-solutions for this service that depends on the business-needs. I usually like to discuss it with my teammates before implementing.

The current solution is a simple **"Lazy Loading"** approach. I chose this because of **low complexity**, **works in scale**, and **fits the requirements**. But here are other approaches that I would have considered:

- **lazy refresh in background**: how? simply return the cached response while we know it is stale, and then trigger a background job to refresh the cache. This way we can ensure that users always get a response.
- **refresh ahead proactively**: how would I do it? by adding a simple scheduling mechanism that refreshes the cache (or even a persistent repository) before/after it expires.
- **Event Driven solution (if possible)**: This is what I started to like more! The upstream service notifies us when a pokemon-entity has been changed/added by sending us an event. Of course in this challenge is not possible, but in a real scenario we might have this option!

---

### Note#2

- **Singleflight**: Fixes the _cache stampede_. Species fetches and translation calls are deduplicated with golang `singleflight`.

- **Architecture**: Ports-and-adapters layout. The `domain` package has zero external dependencies. no I/O, no HTTP. Everything else depends inward on it.

- **Translation as interface**: Both backends (Yoda, Shakespeare) satisfy a single `Translator` interface. Adding new translators requires no changes to the service layer.

---

### what I would have done differently for a production API:

 - In this case, I simply returned a generic error with a message. However, in a production (and complex) system, I would define specific error types for different scenarios to provide more context and enable better error handling. especially using Go’s errors package, which allows wrapping errors.
 - Again, in production, I would have used a framework like `gin` to handle routing, middlewares, authentications, logging, etc. For this code it's not necessary!
 - Note that, I deliberately avoided using a database! obviously the challenge is not about having a repository layer over the API.
 - Yes, I deliberately hard-coded translation baseUrl!
