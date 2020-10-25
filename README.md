# AutoMIDIcally

This is no-where near ready for use yet, but the idea is to be able to control the windows volume sliders for individual applications/devices using real hardware sliders on a MIDI device. I was frustrated with "Power Mixer" disconnecting from my MIDI device and just generally misbehaving and being way more than I really wanted to have for simply mapping a fader to an application.

## Usage
Take a look at the [example config](example_config.yml) to see the configuration parameters. The application is currently only designed for Windows 10, but it will search for MIDI devices with a particular name and then take a mapping of application names (or shell actions) that will control when a signal is received.

Build the application using the [build script](scripts/build.bat) and start it to put the program into the system tray. Most of the operations happen automatically behind the scenes, but the system tray gives a few manual options to reload the configuration, the detected devices, or the detected audio sessions at will.

## Command Line Parameters
`config` - Specify a filepath for the `config.yml` to be read from. Defaults to `config.yml` in the working directory.

`log_level` - Specify the minimum log level required for entries to appear in the log file. Default `info`.

`log_path` - Location to store the logging information. Defaults to `automidically.log` in the working directory.

`notifications` - Wether to use the Windows 10 notification center for Warning and above log messages. Still experimental and due to how the notifications are created may cause some false positives w/ spyware/antivirus software.

`profile_cpu` - A filepath to log the cpu.pprof information from Golang. Default is empty, which disables this from happening.
`profile_memory` - A filepath to log the memory.pprof information from Golang. Default is empty, which disables this from happening.


## Future Ideas
Going to be also looking into triggering automation/scripts/hooks into other applications using MIDI buttons too.
Reuse existing hardware to replicate a stream deck like setup. Hooking into something like an OBS websocket?
