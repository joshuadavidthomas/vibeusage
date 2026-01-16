# vibeusage Specifications

A Python CLI tool for tracking usage across agentic LLM providers. Inspired by CodexBar's architecture but designed for terminal-first workflows.

## Overview

vibeusage provides a unified interface to monitor rate limits, quotas, and costs across multiple AI/LLM service providers. The tool normalizes provider-specific data into consistent data models and presents them through a Rich-powered terminal interface.

## Design Principles

1. **Provider Abstraction**: All providers implement a common interface, producing normalized data structures
2. **Fallback Chains**: Multiple authentication strategies per provider with automatic fallback
3. **Terminal-First**: Optimized for CLI workflows with Rich formatting and JSON output mode
4. **Pace-Based Feedback**: Usage coloring based on consumption pace, not arbitrary thresholds

## Specification Index

| # | Spec | Status | Description |
|---|------|--------|-------------|
| 01 | [Architecture](./01-architecture.md) | **Draft** | Core system design, provider abstraction, fetch pipeline, and module structure |
| 02 | [Data Models](./02-data-models.md) | **Draft** | Normalized data structures for usage, identity, and status |
| 03 | [Authentication](./03-authentication.md) | **Draft** | Authentication strategies, credential types, and fallback chains |
| 04 | [Providers](./04-providers.md) | **Draft** | Provider implementations with API endpoints, auth mechanisms, and response parsing |
| 05 | [CLI Interface](./05-cli-interface.md) | **Draft** | Command structure, Rich rendering, and output formatting |
| 06 | [Configuration](./06-configuration.md) | **Draft** | Settings, credential storage, cache management, and CLI commands |
| 07 | [Error Handling](./07-error-handling.md) | **Draft** | Error classification, retry logic, failure gates, stale data handling, and user-friendly messages |

## Writing Order

Specs are written in dependency order:

```
02-data-models.md      (foundation - defines what we're working with)
        ↓
03-authentication.md   (depends on data models for identity)
        ↓
01-architecture.md     (ties together models + auth into provider abstraction)
        ↓
04-providers.md        (implements the architecture for specific services)
        ↓
05-cli-interface.md    (presents the data)
        ↓
06-configuration.md    (manages credentials and settings)
        ↓
07-error-handling.md   (cross-cutting concern, written last with full context)
```

## Reference Materials

- `../2026-01-15-service-integrations.md` - CodexBar architecture research
- `../ccusage_example.py` - Target CLI output format and UX patterns
- `../PLAN.md` - Planning document with open questions

## Initial Provider Priority

1. **Claude** - Primary target, most complex (OAuth + web + CLI fallbacks)
2. **Codex** - OpenAI/ChatGPT, similar OAuth pattern
3. **Copilot** - GitHub device flow OAuth
4. **Cursor** - Cookie-based, popular IDE
5. **Gemini** - Google OAuth

Later: Augment, Factory, VertexAI, MiniMax, Antigravity, Kiro, Zai
