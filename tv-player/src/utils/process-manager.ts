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
     */
    public killFfmpegProcesses(): Promise<void> {
        return new Promise((resolve, reject) => {
            this.logger.log('Attempting to kill any running ffmpeg processes...');

            this.findFfmpegProcesses().then(pids => {
                if (pids.length === 0) {
                    this.logger.log('No ffmpeg processes were found running.');
                    resolve();
                    return;
                }

                this.logger.log(`Found ${pids.length} ffmpeg processes to kill: ${pids.join(', ')}`);

                exec('pkill -9 ffmpeg || true', (error, stdout, stderr) => {
                    if (error) {
                        if (error.code === 1) {
                            this.logger.log('No ffmpeg processes were found running.');
                            resolve();
                            return;
                        }

                        this.logger.error('Error killing ffmpeg processes:', error);
                        this.logger.error('Stderr:', stderr);
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
     */
    public findFfmpegProcesses(): Promise<number[]> {
        return new Promise((resolve, reject) => {
            // Enhanced command to show command line arguments for better debugging
            exec('ps -eo pid,cmd | grep -i ffmpeg | grep -v grep', (error, stdout, stderr) => {
                if (error) {
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

                const pids: number[] = [];
                const lines = stdout.trim().split('\n');
                const processDetails: string[] = [];

                for (const line of lines) {
                    if (line.trim()) {
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
                    // Log full command lines for better debugging
                    this.logger.log('Process details:');
                    processDetails.forEach((detail, index) => {
                        this.logger.log(`[${index}]: ${detail}`);
                    });
                }

                resolve(pids);
            });
        });
    }

    /**
     * Kill a specific process by PID
     */
    public killProcess(pid: number): Promise<void> {
        return new Promise((resolve, reject) => {
            this.logger.log(`Attempting to kill process with PID: ${pid}...`);

            exec(`ps -p ${pid} -o comm=`, (checkError, checkStdout) => {
                const processName = checkStdout.trim();

                if (checkError) {
                    this.logger.log(`Process with PID ${pid} not found or already terminated.`);
                    resolve();
                    return;
                }

                this.logger.log(`Found process with PID ${pid}: ${processName}`);

                exec(`kill -9 ${pid}`, (error, stdout, stderr) => {
                    if (error) {
                        this.logger.error(`Error killing process ${pid} (${processName}):`, error);
                        this.logger.error('Stderr:', stderr);
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