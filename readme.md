## Note

There would be other design-solutions for this service that depends on the business-needs. I usually like to discuss it with my teammates before implementing.

The current solution is a simple **"Lazy Loading"** approach. I chose this because of **low complexity**, **works in scale**, and **fits the requirements**. But here are other approaches that I would have considered:

- **lazy refresh in background**: how? simply return the cached response while we know it is stale, and then trigger a background job to refresh the cache. This way we can ensure that users always get a response.
- **refresh ahead proactively**: how would I do it? by adding a simple scheduling mechanism that refreshes the cache (or even a persistent repository) before/after it expires.
- **Event Driven solution (if possible)**: This is what I started to like more! The upstream service notifies us when a pokemon-entity has been changed/added by sending us an event. Of course in this challenge is not possible, but in a real scenario we might have this option!

### what I would have done differently for a production API:

 - In this case, I simply returned a generic error with a message. However, in a production (and complex) system, I would define specific error types for different scenarios to provide more context and enable better error handling. especially using Go’s errors package, which allows wrapping errors.
 - Again, in production, I would have used a framework like `gin` to handle routing, middlewares, authentications, logging, etc. For this code it's not necessary!
 - Note that, I deliberately avoided using a database! obviously the challenge is not about having a repository layer over the API.
 - Yes, I deliberately hard-coded translation baseUrl!
