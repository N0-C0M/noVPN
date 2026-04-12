#define MyAppName "NoVPN Desktop"
#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif
#ifndef SourceDir
  #error SourceDir is required (use /DSourceDir=...)
#endif
#ifndef OutputDir
  #define OutputDir "."
#endif

[Setup]
AppId={{8A6AF89C-6F38-4A30-BDA9-6FE247734D94}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
DefaultDirName={autopf}\NoVPN Desktop
DefaultGroupName=NoVPN Desktop
UninstallDisplayIcon={app}\NoVPN Desktop.exe
OutputDir={#OutputDir}
OutputBaseFilename=NoVPN-Desktop-Setup-{#MyAppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin

[Languages]
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: recursesubdirs ignoreversion createallsubdirs

[Icons]
Name: "{group}\NoVPN Desktop"; Filename: "{app}\NoVPN Desktop.exe"
Name: "{autodesktop}\NoVPN Desktop"; Filename: "{app}\NoVPN Desktop.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\NoVPN Desktop.exe"; Description: "Launch NoVPN Desktop"; Flags: nowait postinstall skipifsilent
