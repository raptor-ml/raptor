# Versioning and Releasing for Raptor

- We follow [Semantic Versioning (semver)](https://semver.org)
- The master branch contains all the latest code, some of which may break compatibility.
- We support [PyPi](https://pypi.org/project/raptor-labsdk/) for installing Raptor LabSDK
- We support go modules as our official versioning mechanism.

### Contributors

- Commits and PRs must follows [Conventional Commits](https://www.conventionalcommits.org/) guidelines. For more
  information, see [Contributing](CONTRIBUTING.md)

- Breaking changes will find their way into the next major release, other
  changes will go into a semi-immediate patch or minor release.

- Please try to avoid breaking changes, but if you do, please make sure to
  document te reason well in your PR. We would avoid breaking changes if we can.

## Compatibility

Note that we generally do not support older release branches, except in extreme circumstances.
