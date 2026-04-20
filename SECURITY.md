# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are currently being supported with security updates?

| Version | Supported          |
| ------- | ------------------ |
| 4.x.x   | :white_check_mark: |
| 3.x.x   | :x:                |
| 2.x.x   | :x:                |
| 1.x.x   | :x:                |

## Reporting a Vulnerability

We take the security of PrePump Scanner seriously. If you believe you've found a security vulnerability, please follow these steps:

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Send an email to: **security@yourdomain.com** (replace with actual email)
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce the issue
   - Potential impact
   - Suggested fix (if you have one)
   - Your GitHub username (optional, for follow-up)

### What to Expect

- **Initial Response**: Within 48 hours, you will receive an acknowledgment
- **Status Updates**: We will provide updates every 7 days
- **Resolution Timeline**: We aim to resolve critical issues within 14 days

### Process

1. **Report**: Submit the vulnerability via email
2. **Triage**: We will confirm the vulnerability and determine its severity
3. **Fix**: We will develop and test a fix
4. **Release**: A patched version will be released
5. **Disclosure**: After 30 days from the release, we may publicly disclose the issue

## Security Best Practices

When using PrePump Scanner, please follow these security best practices:

### API Keys and Secrets

- **Never commit** API keys or secrets to the repository
- Use environment variables or secure configuration files
- Rotate keys regularly
- Use separate keys for development and production

```bash
# ✅ Good: Use environment variables
export DEEPCOIN_API_KEY="your_key_here"
./prepump

# ❌ Bad: Hardcoded in config
# deepcoin_key: "your_key_here"  # Don't do this in version control
```

### Configuration Files

- Keep sensitive configuration in `.gitignore`d files
- Use `config.local.yaml` for local settings (not committed)
- Review config before sharing or publishing

### Network Security

- Ensure you're connecting to legitimate API endpoints
- Verify SSL/TLS certificates
- Use a firewall when appropriate
- Monitor network traffic for anomalies

### System Security

- Keep Go and dependencies up to date
- Run with minimal required permissions
- Don't run as root/administrator unless necessary
- Regular system updates

## Known Security Considerations

### Third-Party APIs

PrePump Scanner connects to external services:

- **Pyth Network**: Price feed data
- **Deepcoin**: Market data and trading information
- **Alternative.me**: Fear & Greed Index

Be aware that these services may collect usage data according to their privacy policies.

### Data Storage

- PrePump does not store sensitive data by default
- Cache files are temporary and non-sensitive
- No personal information is collected by the application

### Dependencies

We regularly audit our dependencies for known vulnerabilities:

```bash
# Check for vulnerable dependencies
go list -m -u all
go get -u all

# Review go.sum for unexpected changes
git diff go.sum
```

## Security Updates

Security updates will be released as patch versions (e.g., 4.0.1 → 4.0.2) and announced via:

- GitHub Releases
- Security advisories on GitHub
- Update notifications in the application

## Acknowledgments

We would like to thank the following for their contributions to our security:

- Security researchers who responsibly disclose vulnerabilities
- Community members who report potential issues
- Dependency maintainers who keep our libraries secure

## Contact

For security-related inquiries:
- **Email**: security@yourdomain.com
- **GitHub Security Advisories**: [Link to your security advisories page]

---

**Last Updated**: April 2024
