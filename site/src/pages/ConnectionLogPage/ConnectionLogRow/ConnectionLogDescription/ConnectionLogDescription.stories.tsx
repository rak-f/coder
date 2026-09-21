import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";
import {
	MockConnectedSSHConnectionLog,
	MockDeniedTunnelConnectionLog,
	MockTunnelConnectionLog,
	MockWebConnectionLog,
} from "#/testHelpers/entities";
import { ConnectionLogDescription } from "./ConnectionLogDescription";

const meta: Meta<typeof ConnectionLogDescription> = {
	title: "pages/ConnectionLogPage/ConnectionLogDescription",
	component: ConnectionLogDescription,
};

export default meta;
type Story = StoryObj<typeof ConnectionLogDescription>;

export const SSH: Story = {
	args: {
		connectionLog: MockConnectedSSHConnectionLog,
	},
};

export const App: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
		},
	},
};

export const AppUnauthenticated: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			web_info: {
				...MockWebConnectionLog.web_info!,
				user: null,
			},
		},
	},
};

export const AppAuthenticatedFail: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			web_info: {
				...MockWebConnectionLog.web_info!,
				status_code: 404,
			},
		},
	},
};

export const PortForwardingAuthenticated: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			type: "port_forwarding",
			type_display_name: "Port Forwarding",
			type_family: "port_forwarding",
			web_info: {
				...MockWebConnectionLog.web_info!,
				slug_or_port: "8080",
			},
		},
	},
};

export const AppUnauthenticatedRedirect: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			web_info: {
				...MockWebConnectionLog.web_info!,
				user: null,
				status_code: 303,
			},
		},
	},
};

export const VSCode: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			type: "vscode",
			type_display_name: "VS Code",
			type_family: "vscode",
		},
	},
};

export const Cursor: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			type: "cursor",
			type_display_name: "Cursor",
			type_family: "vscode",
		},
	},
};

export const UnregisteredApp: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			type: "an_unregistered_ide",
			type_display_name: "an_unregistered_ide",
			type_family: "unknown",
		},
	},
};

export const JetBrains: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			type: "jetbrains",
			type_display_name: "JetBrains",
			type_family: "jetbrains",
		},
	},
};

export const Tunnel: Story = {
	args: {
		connectionLog: MockTunnelConnectionLog,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText(/established a tunnel to/)).toBeVisible();
	},
};

// An admin tunneling into another user's workspace, which is the
// primary audit scenario for tunnel events.
export const TunnelOtherUser: Story = {
	args: {
		connectionLog: {
			...MockTunnelConnectionLog,
			workspace_owner_username: "some-other-user",
		},
	},
};

export const TunnelDenied: Story = {
	args: {
		connectionLog: MockDeniedTunnelConnectionLog,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText(/was denied a tunnel to/)).toBeVisible();
	},
};

export const WebTerminal: Story = {
	args: {
		connectionLog: {
			...MockWebConnectionLog,
			type: "reconnecting_pty",
			type_display_name: "Web Terminal",
			type_family: "reconnecting_pty",
		},
	},
};
