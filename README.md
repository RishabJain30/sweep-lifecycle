# Sweep

Garbage collection for your engineering stack.

Sweep is a read-only CLI that identifies temporary engineering resources that may have outlived the development work that created them.

## Status

Sweep is in early development and is not ready for production use.

## Initial scope

Sweep will correlate:

- GitHub pull requests
- Neon database branches
- Vercel preview deployments

The first version will only report cleanup candidates. It will never delete resources.