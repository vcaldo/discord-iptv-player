import ytdl from '@distube/ytdl-core';

export class YoutubeHelper {
    public static async getVideoInternalUrl(videoUrl: string): Promise<string | null> {
        try {
            const video = await ytdl.getInfo(videoUrl, { playerClients: ['WEB', 'ANDROID'] });

            if (video.videoDetails.isLiveContent) {
                const tsFormats = video.formats.filter((format) => format.container === "ts");

                // I'm guessing that bitrate is always present, but if it's not, we'll get an error
                const highestTsFormat = tsFormats.sort((a, b) => b.bitrate! - a.bitrate!)[0];

                return highestTsFormat?.url ?? null;
            }

            return video.formats.filter(format => format.hasVideo && format.hasAudio)[0].url ?? null;
        } catch (error) {
            console.log("Not a valid youtube video url");
            return null;
        }
    }
} 