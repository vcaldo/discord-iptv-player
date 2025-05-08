import { exec } from 'child_process';
import { Logger } from './logger.js';

/**
 * Utility class for managing processes, particularly ffmpeg instances
 * Designed to work in Linux environments (Docker container)
 */
export class ProcessManager {
    private logger: Logger;

    constructor() {
        this.logger = new Logger('ProcessManager');
    }

    /**
     * Kills all ffmpeg processes
     * @returns Promise that resolves when the operation completes
     */
    public killFfmpegProcesses(): Promise<void> {
        return new Promise((resolve, reject) => {
            this.logger.log('Attempting to kill any running ffmpeg processes...');

            // First find the processes to log their PIDs before killing them
            this.findFfmpegProcesses().then(pids => {
                if (pids.length === 0) {
                    this.logger.log('No ffmpeg processes were found running.');
                    resolve();
                    return;
                }

                this.logger.log(`Found ${pids.length} ffmpeg processes to kill: ${pids.join(', ')}`);

                // Using pkill for Linux - kills all processes matching 'ffmpeg'
                exec('pkill -9 ffmpeg || true', (error, stdout, stderr) => {
                    if (error) {
                        // Error code 1 means no processes found, which is not a problem
                        if (error.code === 1) {
                            this.logger.log('No ffmpeg processes were found running.');
                            resolve();
                            return;
                        }

                        this.logger.error('Error killing ffmpeg processes:', error);
                        this.logger.error('Stderr:', stderr);
                        // We resolve instead of reject because we don't want this to block other operations
                        resolve();
                        return;
                    }

                    this.logger.log(`Successfully killed ${pids.length} ffmpeg processes with PIDs: ${pids.join(', ')}`);
                    resolve();
                });
            });
        });
    }

    /**
     * Find ffmpeg processes and return their PIDs
     * @returns Promise that resolves with an array of PIDs
     */
    public findFfmpegProcesses(): Promise<number[]> {
        return new Promise((resolve, reject) => {
            // Using ps and grep to find ffmpeg processes in Linux
            exec('ps -eo pid,comm | grep -i ffmpeg | grep -v grep', (error, stdout, stderr) => {
                if (error) {
                    // Error code 1 just means no processes found (grep didn't match anything)
                    if (error.code === 1) {
                        this.logger.log('No ffmpeg processes found.');
                        resolve([]);
                        return;
                    }

                    this.logger.error('Error finding ffmpeg processes:', error);
                    this.logger.error('Stderr:', stderr);
                    resolve([]);
                    return;
                }

                // Parse the output to extract PIDs
                const pids: number[] = [];
                const lines = stdout.trim().split('\n');
                const processDetails: string[] = [];

                for (const line of lines) {
                    if (line.trim()) {
                        // Format is: "PID COMMAND"
                        const parts = line.trim().split(/\s+/);
                        if (parts.length >= 1) {
                            const pid = parseInt(parts[0], 10);
                            if (!isNaN(pid)) {
                                pids.push(pid);
                                processDetails.push(line.trim());
                            }
                        }
                    }
                }

                if (pids.length === 0) {
                    this.logger.log('No ffmpeg processes found after parsing.');
                } else {
                    this.logger.log(`Found ${pids.length} ffmpeg processes with PIDs: ${pids.join(', ')}`);
                    this.logger.log('Process details:', processDetails);
                }

                resolve(pids);
            });
        });
    }

    /**
     * Kill a specific process by PID
     * @param pid Process ID to kill
     * @returns Promise that resolves when the operation completes
     */
    public killProcess(pid: number): Promise<void> {
        return new Promise((resolve, reject) => {
            this.logger.log(`Attempting to kill process with PID: ${pid}...`);

            // First verify if the process exists
            exec(`ps -p ${pid} -o comm=`, (checkError, checkStdout) => {
                const processName = checkStdout.trim();

                if (checkError) {
                    this.logger.log(`Process with PID ${pid} not found or already terminated.`);
                    resolve();
                    return;
                }

                this.logger.log(`Found process with PID ${pid}: ${processName}`);

                // Using kill command for Linux
                exec(`kill -9 ${pid}`, (error, stdout, stderr) => {
                    if (error) {
                        this.logger.error(`Error killing process ${pid} (${processName}):`, error);
                        this.logger.error('Stderr:', stderr);
                        // We resolve instead of reject because we don't want this to block other operations
                        resolve();
                        return;
                    }

                    this.logger.log(`Successfully killed process ${pid} (${processName})`);
                    resolve();
                });
            });
        });
    }
}