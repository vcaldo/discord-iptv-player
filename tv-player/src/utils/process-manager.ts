import { exec } from 'child_process';
import { Logger } from './logger.js';
import { promisify } from 'util';

const execPromise = promisify(exec);

interface ExecError extends Error {
    code?: number;
    killed?: boolean;
    signal?: string;
    stderr?: string | Buffer;
}

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
                        if ((error as ExecError).code === 1) {
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
            exec('ps -eo pid,comm | grep -i ffmpeg | grep -v grep', (error, stdout, stderr) => {
                if (error) {
                    if ((error as ExecError).code === 1) {
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
                    this.logger.log('Process details:', processDetails);
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

    /**
     * Test a video stream URL to check if it's valid and accessible
     * @param url The video stream URL to test
     * @param timeoutMs Timeout in milliseconds (default: 5000)
     * @returns An object with success status and diagnostic information
     */
    public async testVideoStream(url: string, timeoutMs: number = 5000): Promise<{
        success: boolean;
        details: {
            error?: string;
            format?: string;
            duration?: string;
            resolution?: string;
            codecInfo?: string;
        }
    }> {
        this.logger.log(`Testing video stream: ${url}`);

        try {
            // Run ffprobe with a timeout to check if the stream is accessible
            // -v error: Only show errors
            // -show_entries format=duration,size,bit_rate,format_name : Show basic stream info
            // -show_entries stream=codec_name,width,height : Show codec and resolution info
            // -of json: Output in JSON format
            const cmd = `timeout ${timeoutMs / 1000} ffprobe -v error -show_entries format=duration,size,bit_rate,format_name -show_entries stream=codec_name,width,height -of json "${url}"`;

            this.logger.log(`Running command: ${cmd}`);
            const { stdout, stderr } = await execPromise(cmd);

            if (stderr) {
                this.logger.warn(`FFprobe stderr output: ${stderr}`);
            }

            try {
                const probeData = JSON.parse(stdout);

                // Extract useful information
                const format = probeData.format?.format_name || 'Unknown';
                const duration = probeData.format?.duration || 'N/A';

                // Get video stream information (first video stream)
                const videoStream = probeData.streams?.find((s: any) => s.codec_type === 'video') ||
                                 probeData.streams?.[0]; // Fallback to first stream

                const resolution = videoStream ?
                    `${videoStream.width || 'N/A'}x${videoStream.height || 'N/A'}` :
                    'Unknown';

                const codecInfo = videoStream?.codec_name || 'Unknown';

                this.logger.log(`Stream test successful for ${url}`);
                this.logger.log(`Format: ${format}, Duration: ${duration}, Resolution: ${resolution}, Codec: ${codecInfo}`);

                return {
                    success: true,
                    details: {
                        format,
                        duration,
                        resolution,
                        codecInfo
                    }
                };
            } catch (jsonError) {
                // Could not parse JSON, but we got output
                this.logger.warn(`Error parsing ffprobe output: ${jsonError}`);
                this.logger.log(`Raw ffprobe output: ${stdout}`);

                return {
                    success: !stdout.includes('error') && !stderr.includes('error'),
                    details: {
                        error: `Could not parse ffprobe output: ${(jsonError as Error).message || 'Unknown parsing error'}`,
                        format: stdout.includes('Input #0') ? 'Detected but format unknown' : 'Unknown'
                    }
                };
            }
        } catch (error) {
            this.logger.error(`Error testing video stream: ${error}`);

            let errorMessage = '';
            const execError = error as ExecError;

            if (execError.stderr) {
                errorMessage = execError.stderr.toString();
                this.logger.error(`FFprobe stderr: ${errorMessage}`);
            }

            // Try to extract a meaningful error message
            let diagnosticError = 'Unknown error';
            if (errorMessage.includes('Connection refused')) {
                diagnosticError = 'Connection refused - The server rejected the connection';
            } else if (errorMessage.includes('Connection timed out')) {
                diagnosticError = 'Connection timed out - The server took too long to respond';
            } else if (errorMessage.includes('404')) {
                diagnosticError = 'HTTP 404 - The stream URL was not found on the server';
            } else if (errorMessage.includes('403')) {
                diagnosticError = 'HTTP 403 - Access forbidden, possibly geo-restricted content';
            } else if (errorMessage.includes('No such file')) {
                diagnosticError = 'File not found - The URL might be incorrect';
            } else if (errorMessage.includes('Protocol not found')) {
                diagnosticError = 'Protocol not supported - FFmpeg may need additional protocols enabled';
            } else if (execError.killed || errorMessage.includes('Timeout')) {
                diagnosticError = 'Timeout exceeded - The stream did not respond within the allocated time';
            } else if (errorMessage) {
                diagnosticError = errorMessage.split('\n')[0]; // First line is usually most relevant
            }

            return {
                success: false,
                details: {
                    error: diagnosticError
                }
            };
        }
    }
}