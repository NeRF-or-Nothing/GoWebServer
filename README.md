# VidGoNerf
This is a rewrite of the webserver portion of VidToNerf backend in go. When stress testing V3 of the flask backend, there was serious perfomance issues, and I disliked how it handled SSL and CORS.
In order to make NeRF-Or-Nothing easier to get started on, I will be restructing each backend service into its own repo that is imported as a submodule into a universal backend repo that only contains
docker (and possible nginx) related portions, and then delegates the rest of the build process to each backend service.
