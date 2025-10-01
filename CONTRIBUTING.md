# Contribute

- Create an issue or find an existing [issue](https://gitlab.com/mayachain/mayanode/-/issues)
- About to work on an issue? Start a conversation at #technical channel on [discord](https://discord.gg/mayaprotocol)
- Assign the issue to yourself
- Create a branch using the issue id, for example if the issue you are working on is 600, then create a branch call `600-issue` , this way , gitlab will link your PR with the issue
- Raise a PR , Once your PR is ready for review , post a message in #🖥️community-devs channel in discord , tag `Maya Ah Kin` for review
- Make sure the pipeline is green
- Once PR get approved, you can merge it

Current active branch is `develop` , so when you open PR , make sure your target branch is `develop`

## Vulnerabilities and Bug Bounties

If you find a vulnerability in MAYANode, please submit it for a bounty according to these [guidelines](bugbounty.md).

## the semantic version and release

MAYANode manage changelog entry the same way like gitlab, refer to (https://docs.gitlab.com/ee/development/changelog.html) for more detail. Once a merge request get merged into master branch,
if the merge request upgrade the [version](https://gitlab.com/mayachain/mayanode/-/blob/master/version), then a new release will be created automatically, and the repository will be tagged with
the new version by the release tool.

## New Chain Integration

The process to integrate a new chain into MAYAChain is multifaceted. As it requires changes to multiple repos in multiple languages (`golang`, `python`, and `javascript`).

To learn more about how to add a new chain, follow [this doc](docs/newchain.md)
To learn more about creating your own private chain as a testing and development environment, follow [this doc](docs/private_mock_chain.d)
To learn more about creating your own private chain as a testing and development environment, follow [this doc](docs/private_mock_chain.d)
