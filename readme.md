what I would have done differently for a production API:
 - In this case, I just returned a generic error with a message, but in a production (and complex systems), I would define specific error types for different scenarios to provide more context and allow for better error handling, observability and documenting. Specially with the golang `errors` package which allows you to wrap errors and check for them later on. 
 - Again, in production and in a complex system, I would have used a framework like `gin` to handle routing, middlewares, authentications, logging, etc. For this code it's not necessary!
 - Note that, I deliberately avoided using a database! obviously the challenge is not about having a repository layer over the API.
 - Yes, I deliberately hard-coded translation baseUrl!