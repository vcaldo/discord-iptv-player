import winston from 'winston';
import path from 'path';
import fs from 'fs';
import config from '../config.js';

// Create logs directory if it doesn't exist
const logsDir = path.join(process.cwd(), 'logs');
if (!fs.existsSync(logsDir)) {
  fs.mkdirSync(logsDir, { recursive: true });
}

// Define log format with timestamp, level, context, and message
const logFormat = winston.format.combine(
  winston.format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }),
  winston.format.errors({ stack: true }),
  winston.format.printf(({ timestamp, level, message, context, ...meta }) => {
    const contextStr = context ? `[${context}]` : '';
    const metaStr = Object.keys(meta).length ? JSON.stringify(meta, null, 2) : '';
    return `${timestamp} ${level.toUpperCase()} ${contextStr} ${message} ${metaStr}`;
  })
);

// Create a colorized format for console output
const colorizedFormat = winston.format.combine(
  winston.format.colorize({ all: true }),
  logFormat
);

// Create shared winston transports
const consoleTransport = new winston.transports.Console({
  format: colorizedFormat,
  handleExceptions: true
});

const combinedFileTransport = new winston.transports.File({
  filename: path.join(logsDir, 'combined.log'),
  format: logFormat,
  maxsize: 5242880, // 5MB
  maxFiles: 5,
  tailable: true
});

const errorFileTransport = new winston.transports.File({
  filename: path.join(logsDir, 'error.log'),
  level: 'error',
  format: logFormat,
  maxsize: 5242880, // 5MB
  maxFiles: 5,
  tailable: true
});

// Store all transport instances to access them for flushing
const allTransports = [consoleTransport, combinedFileTransport, errorFileTransport];

/**
 * Enhanced Logger utility class for structured logging
 */
export class Logger {
  private logger: winston.Logger;
  private context: string;

  /**
   * Creates a new Logger instance
   * @param context The logging context (usually class or module name)
   */
  constructor(context: string) {
    this.context = context;

    this.logger = winston.createLogger({
      level: config.isDevelopment() ? 'debug' : 'info',
      defaultMeta: { context },
      transports: allTransports,
      exitOnError: false
    });
  }

  /**
   * Log alias for info level
   */
  public log(message: string, ...meta: any[]): void {
    this.info(message, ...meta);
  }

  /**
   * Log at debug level
   */
  public debug(message: string, ...meta: any[]): void {
    this.logger.debug(message, ...this.formatMeta(meta));
  }

  /**
   * Log at info level
   */
  public info(message: string, ...meta: any[]): void {
    this.logger.info(message, ...this.formatMeta(meta));
  }

  /**
   * Log at warning level
   */
  public warn(message: string, ...meta: any[]): void {
    this.logger.warn(message, ...this.formatMeta(meta));
  }

  /**
   * Log at error level
   */
  public error(message: string, ...meta: any[]): void {
    this.logger.error(message, ...this.formatMeta(meta));
  }

  /**
   * Format metadata objects for better logging
   * @param meta The metadata array to format
   * @returns Formatted metadata
   */
  private formatMeta(meta: any[]): any[] {
    return meta.map(item => {
      // Format Error objects specially to capture stack traces
      if (item instanceof Error) {
        return {
          message: item.message,
          stack: item.stack,
          name: item.name
        };
      }
      return item;
    });
  }
}

/**
 * Flush all log transports to ensure messages are written
 * This is critical during shutdown
 */
export const flushLogs = async (): Promise<void> => {
  return new Promise<void>((resolve) => {
    appLogger.info('[SHUTDOWN] Flushing log transports...');

    // Set a fallback timeout in case flushing gets stuck
    const fallbackTimeout = setTimeout(() => {
      console.log('[SHUTDOWN] Log flush timeout - forced completion');
      resolve();
    }, 2000);

    // Count completed transports
    let completed = 0;

    // Nothing to flush
    if (allTransports.length === 0) {
      clearTimeout(fallbackTimeout);
      resolve();
      return;
    }

    // Try to flush each transport
    allTransports.forEach(transport => {
      // Check if transport has a 'flush' method
      if (typeof (transport as any).flush === 'function') {
        try {
          (transport as any).flush(() => {
            completed++;
            if (completed >= allTransports.length) {
              clearTimeout(fallbackTimeout);
              resolve();
            }
          });
        } catch (err) {
          console.error('[SHUTDOWN] Error flushing transport:', err);
          completed++;
          if (completed >= allTransports.length) {
            clearTimeout(fallbackTimeout);
            resolve();
          }
        }
      } else {
        // No flush method, just count it as completed
        completed++;
        if (completed >= allTransports.length) {
          clearTimeout(fallbackTimeout);
          resolve();
        }
      }
    });
  });
};

// Create a singleton instance for application-wide logging
export const appLogger = new Logger('App');

// Export simple log functions for quick access
export const logDebug = (msg: string, ...meta: any[]) => appLogger.debug(msg, ...meta);
export const logInfo = (msg: string, ...meta: any[]) => appLogger.info(msg, ...meta);
export const logWarn = (msg: string, ...meta: any[]) => appLogger.warn(msg, ...meta);
export const logError = (msg: string, ...meta: any[]) => appLogger.error(msg, ...meta);