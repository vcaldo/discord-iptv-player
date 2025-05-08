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

    // Create a logger instance with console and file transports
    this.logger = winston.createLogger({
      level: config.isDevelopment() ? 'debug' : 'info',
      defaultMeta: { context },
      transports: [
        // Console transport with colors
        new winston.transports.Console({
          format: colorizedFormat
        }),
        // File transport for all logs
        new winston.transports.File({
          filename: path.join(logsDir, 'combined.log'),
          format: logFormat,
          maxsize: 5242880, // 5MB
          maxFiles: 5,
          tailable: true
        }),
        // Separate file for error logs
        new winston.transports.File({
          filename: path.join(logsDir, 'error.log'),
          level: 'error',
          format: logFormat,
          maxsize: 5242880, // 5MB
          maxFiles: 5,
          tailable: true
        })
      ]
    });
  }

  /**
   * Log an informational message
   * @param message The message to log
   * @param meta Optional metadata to include
   */
  public log(message: string, ...meta: any[]): void {
    this.info(message, ...meta);
  }

  /**
   * Log debug information (only in development)
   * @param message The message to log
   * @param meta Optional metadata to include
   */
  public debug(message: string, ...meta: any[]): void {
    this.logger.debug(message, ...this.formatMeta(meta));
  }

  /**
   * Log informational message
   * @param message The message to log
   * @param meta Optional metadata to include
   */
  public info(message: string, ...meta: any[]): void {
    this.logger.info(message, ...this.formatMeta(meta));
  }

  /**
   * Log a warning message
   * @param message The message to log
   * @param meta Optional metadata to include
   */
  public warn(message: string, ...meta: any[]): void {
    this.logger.warn(message, ...this.formatMeta(meta));
  }

  /**
   * Log an error message
   * @param message The message to log
   * @param meta Optional metadata to include (error objects, etc.)
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

// Create a singleton instance for application-wide logging
export const appLogger = new Logger('App');

// Export simple log functions for quick access
export const logDebug = (msg: string, ...meta: any[]) => appLogger.debug(msg, ...meta);
export const logInfo = (msg: string, ...meta: any[]) => appLogger.info(msg, ...meta);
export const logWarn = (msg: string, ...meta: any[]) => appLogger.warn(msg, ...meta);
export const logError = (msg: string, ...meta: any[]) => appLogger.error(msg, ...meta);