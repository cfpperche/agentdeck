import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Chat component: the critical interactive flows (queue tags, permission
// banner with edit, state badges). api.js mocked at module level.

vi.mock("./api.js", () => ({
  api: {
    messages: vi.fn().mockResolvedValue([
      { id: 1, role: "user", content: "hi" },
      { id: 2, role: "assistant", content: "hello", meta: null },
    ]),
    send: vi.fn().mockResolvedValue({ ok: true, queued: false }),
    stop: vi.fn().mockResolvedValue({}),
    clearQueue: vi.fn().mockResolvedValue({}),
    control: vi.fn().mockResolvedValue({ ok: true }),
  },
  openEvents: vi.fn().mockReturnValue({
    close: vi.fn(),
    // expose the handler the component registered so tests can push events
    _handler: null,
  }),
}));

import { Chat } from "./chat.jsx";
import { api, openEvents } from "./api.js";

const session = { id: "s1", agent: "claude", title: "T", cwd: "" };
const agentMeta = { id: "claude", label: "Claude", color: "#E07856" };

function lastHandler() {
  return openEvents.mock.results[openEvents.mock.results.length - 1].value;
}

// patch: openEvents must capture the handler
beforeAll(() => {
  openEvents.mockImplementation((sid, h) => {
    openEvents._handler = h;
    return { close: vi.fn() };
  });
});

async function pushEvent(ev) {
  await act(async () => openEvents._handler?.(ev));
}

describe("Chat", () => {
  it("renders persisted messages", async () => {
    render(<Chat session={session} agentMeta={agentMeta} />);
    expect(await screen.findByText("hello")).toBeInTheDocument();
    expect(screen.getByText("hi")).toBeInTheDocument();
  });

  it("sends a message and optimistically appends it", async () => {
    const user = userEvent.setup();
    render(<Chat session={session} agentMeta={agentMeta} />);
    const box = await screen.findByRole("textbox");
    await user.type(box, "do the thing{Enter}");
    expect(api.send).toHaveBeenCalledWith("s1", "do the thing");
    expect(await screen.findByText("do the thing")).toBeInTheDocument();
  });

  it("queued messages show the queued tag", async () => {
    api.send.mockResolvedValueOnce({ ok: true, queued: true });
    const user = userEvent.setup();
    render(<Chat session={session} agentMeta={agentMeta} />);
    const box = await screen.findByRole("textbox");
    await user.type(box, "steer{Enter}");
    expect(await screen.findByText(/queued — will send/i)).toBeInTheDocument();
  });

  it("permission banner: queue counter, edit, allow with edits", async () => {
    const user = userEvent.setup();
    render(<Chat session={session} agentMeta={agentMeta} />);
    await screen.findByText("hello");

    await pushEvent({ type: "permission", request_id: "r1", tool: "Bash", input: '{"command":"rm -rf /"}' });
    await pushEvent({ type: "permission", request_id: "r2", tool: "Write", input: '{"file_path":"/x"}' });

    expect(await screen.findByText(/Bash permission requested/i)).toBeInTheDocument();
    expect(screen.getByText(/1 of 2/)).toBeInTheDocument();

    // edit the input and allow with edits
    await user.click(screen.getByText("edit input"));
    const ta = screen.getByDisplayValue(/rm -rf/);
    await user.clear(ta);
    await user.paste('{"command":"echo safe"}');
    await user.click(screen.getByText("Allow with edits"));

    expect(api.control).toHaveBeenCalledWith(
      "s1", "r1", "allow", { command: "echo safe" }
    );
    // next in queue becomes active
    expect(await screen.findByText(/Write permission requested/i)).toBeInTheDocument();
  });

  it("deny path calls control with deny", async () => {
    const user = userEvent.setup();
    render(<Chat session={session} agentMeta={agentMeta} />);
    await screen.findByText("hello");
    await pushEvent({ type: "permission", request_id: "r9", tool: "Bash", input: "{}" });
    await user.click(await screen.findByText("Deny"));
    expect(api.control).toHaveBeenCalledWith("s1", "r9", "deny", undefined);
  });

  it("running/waiting badges reflect state events", async () => {
    render(<Chat session={session} agentMeta={agentMeta} />);
    await screen.findByText("hello");
    await pushEvent({ type: "state", running: true, status: "running" });
    expect(await screen.findByText("running")).toBeInTheDocument();
    await pushEvent({ type: "state", running: true, status: "waiting" });
    expect(await screen.findByText("waiting")).toBeInTheDocument();
  });
});
