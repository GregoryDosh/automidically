# AutoMIDIcally

![nanoKONTROL2](/assets/nanoKONTROL2%20Example.png "Example of a nanoKONTROL2 with faders labeled with icons.")

AutoMIDIcally gives Windows users an ability to map buttons, faders, etc. on a MIDI device to actions within windows. The major feature is being able to control the volumes for applications independently by mapping real faders to the same applications you'd see in the Volume Mixer within Windows. There is also some small integrations with shell actions based on button inputs, so if you can control it from the command line you should be able to control it through your MIDI device.

The reason I made this is because I was frustrated with "Power Mixer" disconnecting from my MIDI device and just generally misbehaving and being way more than I really wanted to have for simply mapping a fader to an application. It also didn't allow the flexibility that I really wanted to have with easily mapping multiple applications to a mixer without multiple menus and duplicating GUI settings, it wasn't as quick as open text file, edit, save.

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

## Known Issues
- I believe it a bug in either rtmidi (via gitlab.com/gomidi/rtmididrv) or the library itself, but if you run AutoMIDIcally as an administrator then your computer has a chance that it won't go to sleep properly and will hang/crash instead. At least it does for me, even removing all of the components of this repo and running their example directly hooked to real device and it stops my computer from sleeping when run as an admin.

## Future Ideas
- Going to be also looking into triggering automation/scripts/hooks into other applications using MIDI buttons too. (Most of this can just be done with the shell component and powershell commands ATM)
- Reuse existing hardware to replicate a stream deck like setup. Hooking into something like an OBS websocket?
