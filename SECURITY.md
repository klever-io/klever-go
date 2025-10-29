# Security Policy

## Overview

The Klever blockchain team takes security vulnerabilities seriously. We appreciate your efforts to responsibly disclose your findings and will make every effort to acknowledge your contributions.

## Supported Versions

We actively support and provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.7.x   | :white_check_mark: |
| < 1.7.0 | :x:                |

**Note:** We strongly recommend using the latest stable release to ensure you have the most recent security patches and improvements.

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues, discussions, or pull requests.**

Instead, please report security vulnerabilities using one of the following methods:

### Private Security Advisory (Recommended)

Report vulnerabilities through GitHub's private vulnerability reporting:
1. Navigate to the **Security** tab of this repository
2. Click **Report a vulnerability**
3. Fill out the vulnerability details form

### Email

Send details to: **security@klever.org**

Please include the following information in your report:

- **Type of vulnerability** (e.g., consensus failure, smart contract execution bypass, DoS, etc.)
- **Affected component(s)** (e.g., KVM, consensus mechanism, networking layer)
- **Step-by-step instructions** to reproduce the issue
- **Proof of concept** or exploit code (if available)
- **Potential impact** of the vulnerability
- **Suggested mitigation** (if you have one)
- **Your contact information** for follow-up questions

## Vulnerability Severity Classification

We use the following severity levels to classify security issues:

### Critical
- Consensus failures or chain halts (BLS-based slot consensus with Byzantine fault tolerance)
- Unauthorized fund access or theft
- Remote code execution
- Private key exposure
- Byzantine attacks affecting consensus integrity

### High
- Denial of Service affecting network availability
- Smart contract execution vulnerabilities
- Authentication/authorization bypass
- Transaction validation bypass

### Medium
- Information disclosure
- Performance degradation attacks
- Non-critical DoS vectors

### Low
- Issues with limited impact
- Best practice violations
- Security improvements

## Response Timeline

We are committed to addressing security vulnerabilities promptly:

1. **Initial Response**: Within 48 hours of receiving your report
2. **Triage and Assessment**: Within 5 business days
3. **Fix Development**: Depending on complexity and severity
   - Critical: 7-14 days
   - High: 14-30 days
   - Medium: 30-60 days
   - Low: 60-90 days
4. **Coordinated Disclosure**: We will work with you to determine an appropriate disclosure timeline

## Security Update Process

When a security vulnerability is confirmed:

1. We will develop and test a fix
2. We will prepare security advisories
3. We will notify affected users and node operators through official channels
4. We will release the patched version
5. After a reasonable adoption period, we will publish the security advisory with credit to the reporter (if desired)

## Bug Bounty Program

We value the security research community's contributions. Details about our bug bounty program:

- **Scope**: Vulnerabilities in the core blockchain protocol, consensus mechanism, KVM, smart contract execution, and cryptographic implementations
- **Rewards**: Determined based on severity and impact (see classification above)
- **Eligibility**: Must follow responsible disclosure practices

For current bounty amounts and specific program details, please contact **security@klever.org**.

## Out of Scope

The following are generally considered out of scope:

- Issues in third-party dependencies (please report to the respective maintainers)
- Social engineering attacks
- Physical attacks on infrastructure
- Vulnerabilities requiring unlikely user interaction
- Issues already reported or fixed
- Automated scanning results without proof of exploitability

## Responsible Disclosure Guidelines

When researching vulnerabilities, please:

- ✅ Make every effort to avoid privacy violations, data destruction, and service disruption
- ✅ Only interact with accounts you own or have explicit permission to test
- ✅ Do not exploit vulnerabilities beyond what is necessary to demonstrate the issue
- ✅ Keep all vulnerability details confidential until they are resolved
- ✅ Give us reasonable time to fix vulnerabilities before public disclosure

Please **do not**:

- ❌ Access, modify, or delete data that doesn't belong to you
- ❌ Perform DoS/DDoS attacks on the mainnet or public testnet
- ❌ Compromise user privacy or degrade user experience
- ❌ Execute attacks against network participants
- ❌ Publicly disclose vulnerabilities before coordinated release

## Security Best Practices for Users

To help secure the Klever blockchain ecosystem:

- Keep your node software up to date
- Follow secure key management practices
- Use hardware wallets for significant holdings
- Verify transaction details before signing
- Be cautious of social engineering attempts
- Report suspicious activity to the team

## Security Audits

Our codebase undergoes regular security audits by reputable third-party firms. Audit reports are published on our website and documentation.

## Contact

For any security-related questions or concerns:

- **Email**: security@klever.org
- **Website**: https://klever.org
- **Documentation**: https://docs.klever.org

## Acknowledgments

We would like to thank the security researchers and community members who help keep Klever safe. Contributors who follow responsible disclosure practices will be acknowledged (with permission) in our security advisories.

---

**Last Updated**: October 2025
