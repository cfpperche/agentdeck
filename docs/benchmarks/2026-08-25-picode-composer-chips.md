# Study: PiCode composer chips (pi surface)

- **Date:** 2026-08-25
- **Scope:** give the Pi runtime the same five chips PiCode ships.
  Other runtimes stay on the existing model/thinking/permission strip
  until we survey each protocol.

## PiCode receipts

- `web/src/components/{Provider,Model,Thinking,Mode,Kind}Chip.jsx`
- `web/src/lib/chip.js` — provider list from catalog (signed-in),
  models scoped to provider, thinking levels per model, Full vs
  Read-only (`--tools read,grep,find,ls`), Prompt/Steer/Follow-up.
- `docs/rpc.md` (pi): `set_model` `{provider, modelId}`,
  `set_thinking_level`, commands `prompt` / `steer` / `follow_up`.
- Read-only is spawn-time (`--tools`), not an RPC method.

## AgentDeck

- Live `get_available_models` is grouped into `capabilities.providers`.
- Composer shows Provider / Model / Thinking / Full / Prompt when
  those fields exist (pi DefaultCaps + live catalog).
- `controls.kind` becomes the RPC command type.
- `controls.op_mode=readonly` sets `AGENTDECK_PI_TOOLS` on the next
  spawn. Mid-session switch waits for the next process start.
