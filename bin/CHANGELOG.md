## Changelog
* 007984b179a02851f660b7b884a9eb2305589f33 Added a seperate bin directory to handle helm plugin install without getting in the way of goreleaser
* ce4afa71457ce46c30e8a7e2e6b40032dbd5deca Added platform specific commands
* 5c11fdd925eff079a3d35d76ffef8be6672b1cf8 Adding bin and removing dist so we can support multiple platforms
* eb02870b6b5cf5d607d5c32384e35e1a810df9be Adding bin and removing dist so we can support multiple platforms
* 184ded59264884b7199acd9b7feb3796e8929a19 Adding dist directory
* bcbf964f171976a9185ebea63f2f2a989de86f27 Correct plugin name in INSTALL.md
* 79085101a10f56679b24ca62beef2cebaae5262b Fixed tabs
* 62f9caf4dd868a916ec58a7463be61ad58d6fefd Removed snapshot and archive sections, these are deprecated anyway and aren't needed for helm distribution
* 99959712cb6b6839d977a9e364768078866fa575 Removing dist directory as we can't check this in cause it causes goreleaser to fail
* 32c9352a2b40c215341de68efcde3d58ba97f717 Since we're checking in the dist directory, need to make goreleaser ignore it
* acd4822af8d40e3b297b4af8a70fe4f81b2afa9b Since we're checking in the dist directory, need to make goreleaser ignore it
* eed3f013ec95aef462464bd2950181225d4b4ed4 Syncing up the package, module, binary, etc. names with the repo name.
* d0a5f3a3c109cd059ecf35fe6449a0eb54a6897a Turning off archive generation
* 355d738906da9deb24f9421ea33ba7438bef06ab Turning off archive generation
* 1f248bc488d2f807222d2fe00d15a1c54e63f27e Update command path in plugin.yaml
* ed353005af3f76552eba49e038f41f79d4f3e439 Update repository name and references in CHANGELOG
* f8c925560e2427661cd571477fd1b3fec4fed88c Updated change log section to reference the CHANGELOG.md
* 99ac49b9939ebf6789830b3b168ea644de61cf6d Updated name to match the new repo name
* 12d30e275bdcbceb21c53f6a7cd1fb4b010a26ab Updated repo name to match plugin name.
* bcef0d7b2a5b7128b7318c2da1fa38a81a743d36 Updated to use the bin directory instead of dist as committing the dist directory messes up goreleaser
* 9aaf564b135e76f8fcf0b9e75abd83a0f25dece9 Updating version to 1.0.1
