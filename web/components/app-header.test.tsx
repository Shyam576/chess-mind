import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { AppHeader } from "./app-header";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: pushMock }) }));
// ThemeToggle reads window.matchMedia, which jsdom does not implement; it is
// not under test here.
vi.mock("./theme-toggle", () => ({ ThemeToggle: () => null }));

describe("AppHeader", () => {
  beforeEach(() => {
    pushMock.mockClear();
    global.fetch = vi.fn();
  });

  it("renders links to start a new game and view history", () => {
    render(<AppHeader />);
    expect(screen.getByRole("link", { name: /new game/i })).toHaveAttribute("href", "/play");
    expect(screen.getByRole("link", { name: /history/i })).toHaveAttribute("href", "/games");
  });

  it("logs out and redirects to /login", async () => {
    vi.mocked(global.fetch).mockResolvedValue({ ok: true, status: 204, json: async () => ({}) } as Response);
    render(<AppHeader />);
    fireEvent.click(screen.getByRole("button", { name: /log out/i }));
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/login"));
  });
});
