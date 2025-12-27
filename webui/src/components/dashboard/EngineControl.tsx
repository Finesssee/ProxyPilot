import { useState, useEffect, useRef } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
	Play,
	Square,
	RotateCw,
	FolderOpen,
	FileText,
	Copy,
	Activity,
	Check,
	AlertTriangle,
	Radio,
} from "lucide-react";
import { useProxyContext } from "@/hooks/useProxyContext";

function formatUptime(seconds: number): string {
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	const s = seconds % 60;
	if (h > 0) {
		return `${h}h ${m}m`;
	}
	if (m > 0) {
		return `${m}m ${s}s`;
	}
	return `${s}s`;
}

export function EngineControl() {
	const { status, loading, handleAction, showToast, isDesktop } = useProxyContext();
	const [uptime, setUptime] = useState(0);
	const [copied, setCopied] = useState(false);
	const startTimeRef = useRef<number | null>(null);

	const isRunning = status?.running ?? false;

	// Track uptime
	useEffect(() => {
		if (isRunning) {
			if (startTimeRef.current === null) {
				startTimeRef.current = Date.now();
			}
			const interval = setInterval(() => {
				if (startTimeRef.current) {
					setUptime(Math.floor((Date.now() - startTimeRef.current) / 1000));
				}
			}, 1000);
			return () => clearInterval(interval);
		}
		startTimeRef.current = null;
		setUptime(0);
	}, [isRunning]);

	const handleCopyUrl = async () => {
		if (status?.base_url) {
			try {
				await navigator.clipboard.writeText(status.base_url);
				setCopied(true);
				showToast("URL copied to clipboard", "success");
				setTimeout(() => setCopied(false), 2000);
			} catch {
				showToast("Failed to copy URL", "error");
			}
		}
	};

	return (
		<div className="rounded-lg border border-[var(--border-default)] bg-[var(--bg-panel)] shadow-xl overflow-hidden">
			{/* Flight Deck Header */}
			<div className="flex items-center gap-2 px-4 py-3 bg-[var(--bg-elevated)] border-b border-[var(--border-default)]">
				<Radio className="h-4 w-4 text-[var(--accent-primary)]" />
				<span className="font-mono text-xs font-bold uppercase tracking-widest text-[var(--text-secondary)]">
					Engine Control
				</span>
				<div className="flex-1" />
				<div className={`h-2 w-2 rounded-full ${isRunning ? "bg-[var(--status-online)] animate-pulse" : "bg-[var(--status-offline)]"}`} />
			</div>

			{/* Radar Display Area */}
			<div className="p-6 flex flex-col items-center">
				{/* Radar Container */}
				<div className="relative w-48 h-48 mb-6">
					{/* Outer pulse rings - only visible when running */}
					{isRunning && (
						<>
							<div
								className="absolute inset-0 rounded-full border-2 border-[var(--status-online)] opacity-0"
								style={{
									animation: "radar-pulse 2s ease-out infinite",
								}}
							/>
							<div
								className="absolute inset-0 rounded-full border-2 border-[var(--status-online)] opacity-0"
								style={{
									animation: "radar-pulse 2s ease-out infinite 0.5s",
								}}
							/>
							<div
								className="absolute inset-0 rounded-full border-2 border-[var(--status-online)] opacity-0"
								style={{
									animation: "radar-pulse 2s ease-out infinite 1s",
								}}
							/>
						</>
					)}

					{/* Static outer ring */}
					<div
						className={`absolute inset-4 rounded-full border-2 transition-colors duration-300 ${
							isRunning
								? "border-[var(--status-online)]/30"
								: "border-[var(--border-subtle)]"
						}`}
					/>

					{/* Middle ring with tick marks */}
					<div
						className={`absolute inset-8 rounded-full border transition-colors duration-300 ${
							isRunning
								? "border-[var(--status-online)]/20"
								: "border-[var(--border-subtle)]/50"
						}`}
					/>

					{/* Inner radar circle - main status display */}
					<div
						className={`absolute inset-12 rounded-full flex flex-col items-center justify-center transition-all duration-500 ${
							isRunning
								? "bg-gradient-to-br from-[var(--accent-primary)] to-[var(--accent-secondary)] shadow-[0_0_30px_var(--status-online)]"
								: "bg-gradient-to-br from-[var(--bg-elevated)] to-[var(--bg-active)] border border-[var(--border-default)]"
						}`}
					>
						{/* Status text */}
						<span
							className={`font-mono text-xs font-bold tracking-wider ${
								isRunning
									? "text-white"
									: "text-[var(--text-muted)]"
							}`}
						>
							{isRunning ? "ONLINE" : "OFFLINE"}
						</span>
					</div>

					{/* Rotating sweep line - only when running */}
					{isRunning && (
						<div
							className="absolute inset-0 rounded-full overflow-hidden"
							style={{
								animation: "radar-sweep 3s linear infinite",
							}}
						>
							<div
								className="absolute top-1/2 left-1/2 w-1/2 h-0.5 origin-left"
								style={{
									background: "linear-gradient(90deg, var(--status-online) 0%, transparent 100%)",
								}}
							/>
						</div>
					)}
				</div>

				{/* Instrument Readouts */}
				<div className="w-full grid grid-cols-2 gap-3 mb-6">
					{/* Port Readout */}
					<div className="bg-[var(--bg-void)] rounded-md border border-[var(--border-subtle)] p-3">
						<div className="font-mono text-[10px] uppercase tracking-wider text-[var(--text-muted)] mb-1">
							Port
						</div>
						<div className={`font-mono text-lg font-bold ${isRunning ? "text-[var(--accent-glow)]" : "text-[var(--text-muted)]"}`}>
							{isRunning ? status?.port || "---" : "---"}
						</div>
					</div>

					{/* Uptime Readout */}
					<div className="bg-[var(--bg-void)] rounded-md border border-[var(--border-subtle)] p-3">
						<div className="font-mono text-[10px] uppercase tracking-wider text-[var(--text-muted)] mb-1">
							Uptime
						</div>
						<div className={`font-mono text-lg font-bold ${isRunning ? "text-[var(--accent-glow)]" : "text-[var(--text-muted)]"}`}>
							{isRunning ? formatUptime(uptime) : "--:--"}
						</div>
					</div>
				</div>

				{/* Error Display - Warning Panel */}
				{status?.last_error && (
					<div className="w-full mb-6 rounded-md border-2 border-[var(--status-warning)] bg-[var(--status-warning)]/10 p-4">
						<div className="flex items-start gap-3">
							<div className="p-1.5 rounded bg-[var(--status-warning)]/20">
								<AlertTriangle className="h-4 w-4 text-[var(--status-warning)]" />
							</div>
							<div className="flex-1 min-w-0">
								<div className="font-mono text-[10px] uppercase tracking-wider text-[var(--status-warning)] mb-1">
									Caution
								</div>
								<div className="font-mono text-sm text-[var(--status-warning)] break-all">
									{status.last_error}
								</div>
							</div>
						</div>
					</div>
				)}

				{/* Cockpit Control Buttons */}
				<div className="w-full grid grid-cols-3 gap-3 mb-6">
					{/* START Button */}
					<Tooltip>
						<TooltipTrigger asChild>
							<button
								type="button"
								disabled={isRunning || loading === "start"}
								onClick={() => handleAction(window.pp_start, "start", "Proxy started")}
								className={`
									relative flex flex-col items-center justify-center gap-1 py-4 px-3 rounded-lg
									font-mono text-xs font-bold uppercase tracking-wider
									border-2 transition-all duration-150
									${isRunning || loading === "start"
										? "opacity-40 cursor-not-allowed border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-muted)]"
										: "border-[var(--status-online)] bg-gradient-to-b from-[var(--bg-elevated)] to-[var(--bg-panel)] text-[var(--status-online)] hover:shadow-[0_0_15px_var(--status-online)] active:translate-y-0.5 active:shadow-none cursor-pointer"
									}
									shadow-[inset_0_1px_0_rgba(255,255,255,0.1),0_2px_4px_rgba(0,0,0,0.3)]
								`}
							>
								<Play className="h-5 w-5 fill-current" />
								<span>Start</span>
							</button>
						</TooltipTrigger>
						<TooltipContent>Start the proxy server</TooltipContent>
					</Tooltip>

					{/* STOP Button */}
					<Tooltip>
						<TooltipTrigger asChild>
							<button
								type="button"
								disabled={!isRunning || loading === "stop"}
								onClick={() => handleAction(window.pp_stop, "stop", "Proxy stopped")}
								className={`
									relative flex flex-col items-center justify-center gap-1 py-4 px-3 rounded-lg
									font-mono text-xs font-bold uppercase tracking-wider
									border-2 transition-all duration-150
									${!isRunning || loading === "stop"
										? "opacity-40 cursor-not-allowed border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-muted)]"
										: "border-[var(--status-offline)] bg-gradient-to-b from-[var(--bg-elevated)] to-[var(--bg-panel)] text-[var(--status-offline)] hover:shadow-[0_0_15px_var(--status-offline)] active:translate-y-0.5 active:shadow-none cursor-pointer"
									}
									shadow-[inset_0_1px_0_rgba(255,255,255,0.1),0_2px_4px_rgba(0,0,0,0.3)]
								`}
							>
								<Square className="h-5 w-5 fill-current" />
								<span>Stop</span>
							</button>
						</TooltipTrigger>
						<TooltipContent>Stop the proxy server</TooltipContent>
					</Tooltip>

					{/* RESTART Button */}
					<Tooltip>
						<TooltipTrigger asChild>
							<button
								type="button"
								disabled={!isRunning || loading === "restart"}
								onClick={() => handleAction(window.pp_restart, "restart", "Proxy restarted")}
								className={`
									relative flex flex-col items-center justify-center gap-1 py-4 px-3 rounded-lg
									font-mono text-xs font-bold uppercase tracking-wider
									border-2 transition-all duration-150
									${!isRunning || loading === "restart"
										? "opacity-40 cursor-not-allowed border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-muted)]"
										: "border-[var(--accent-primary)] bg-gradient-to-b from-[var(--bg-elevated)] to-[var(--bg-panel)] text-[var(--accent-primary)] hover:shadow-[0_0_15px_var(--accent-primary)] active:translate-y-0.5 active:shadow-none cursor-pointer"
									}
									shadow-[inset_0_1px_0_rgba(255,255,255,0.1),0_2px_4px_rgba(0,0,0,0.3)]
								`}
							>
								<RotateCw className={`h-5 w-5 ${loading === "restart" ? "animate-spin" : ""}`} />
								<span>Restart</span>
							</button>
						</TooltipTrigger>
						<TooltipContent>Restart the proxy server</TooltipContent>
					</Tooltip>
				</div>

				{/* Base URL Display */}
				{status?.base_url && (
					<div className="w-full bg-[var(--bg-void)] rounded-md border border-[var(--border-subtle)] p-3">
						<div className="flex items-center justify-between gap-3">
							<div className="flex-1 min-w-0">
								<div className="font-mono text-[10px] uppercase tracking-wider text-[var(--text-muted)] mb-1">
									Base URL
								</div>
								<div className="font-mono text-sm text-[var(--text-accent)] truncate">
									{status.base_url}
								</div>
							</div>
							<button
								type="button"
								onClick={handleCopyUrl}
								className={`
									flex items-center gap-1.5 px-3 py-1.5 rounded
									font-mono text-xs uppercase tracking-wider
									border transition-all duration-150
									${copied
										? "border-[var(--status-online)] bg-[var(--status-online)]/10 text-[var(--status-online)]"
										: "border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)]"
									}
									active:translate-y-0.5 cursor-pointer
								`}
							>
								{copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
								{copied ? "Copied" : "Copy"}
							</button>
						</div>
					</div>
				)}

				{/* Quick Actions - Desktop Only */}
				{isDesktop && (
					<div className="w-full mt-4 pt-4 border-t border-[var(--border-subtle)]">
						<div className="font-mono text-[10px] uppercase tracking-wider text-[var(--text-muted)] mb-3">
							Quick Actions
						</div>
						<div className="flex flex-wrap gap-2">
							<button
								type="button"
								onClick={() => handleAction(window.pp_open_logs, "logs", "Opened logs")}
								className="flex items-center gap-1.5 px-3 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] font-mono text-xs hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)] transition-colors active:translate-y-0.5 cursor-pointer"
							>
								<FileText className="h-3.5 w-3.5" />
								Logs
							</button>

							<button
								type="button"
								onClick={() => handleAction(window.pp_open_auth_folder, "auth", "Opened auth folder")}
								className="flex items-center gap-1.5 px-3 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] font-mono text-xs hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)] transition-colors active:translate-y-0.5 cursor-pointer"
							>
								<FolderOpen className="h-3.5 w-3.5" />
								Auth
							</button>

							<button
								type="button"
								disabled={!isRunning}
								onClick={() => handleAction(window.pp_open_diagnostics, "diag-open", "Opened diagnostics")}
								className={`flex items-center gap-1.5 px-3 py-1.5 rounded border font-mono text-xs transition-colors active:translate-y-0.5 cursor-pointer ${
									!isRunning
										? "opacity-40 cursor-not-allowed border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-muted)]"
										: "border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)]"
								}`}
							>
								<Activity className="h-3.5 w-3.5" />
								Diagnostics
							</button>
						</div>
					</div>
				)}

				{/* Non-desktop Copy URL */}
				{!isDesktop && !status?.base_url && (
					<div className="w-full mt-4 pt-4 border-t border-[var(--border-subtle)]">
						<button
							type="button"
							onClick={handleCopyUrl}
							disabled={!status?.base_url}
							className="flex items-center gap-1.5 px-3 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] font-mono text-xs hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)] transition-colors disabled:opacity-40 disabled:cursor-not-allowed active:translate-y-0.5 cursor-pointer"
						>
							{copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
							{copied ? "Copied" : "Copy URL"}
						</button>
					</div>
				)}
			</div>

			{/* CSS for radar animations */}
			<style>{`
				@keyframes radar-pulse {
					0% {
						transform: scale(0.8);
						opacity: 0.8;
					}
					100% {
						transform: scale(1.5);
						opacity: 0;
					}
				}

				@keyframes radar-sweep {
					0% {
						transform: rotate(0deg);
					}
					100% {
						transform: rotate(360deg);
					}
				}
			`}</style>
		</div>
	);
}
