#define AppName "ProxyPilot"
#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#ifndef RepoRoot
  #define RepoRoot ".."
#endif
#ifndef OutDir
  #define OutDir "..\\dist"
#endif

[Setup]
; --- App Identity ---
AppId={{3D9053A0-3F6A-47D7-9D91-8BB1D1CC2A4E}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher=ProxyPilot
AppPublisherURL=https://github.com/Finesssee/ProxyPilot
AppSupportURL=https://github.com/Finesssee/ProxyPilot/issues
AppUpdatesURL=https://github.com/Finesssee/ProxyPilot/releases
AppComments=Local AI Proxy with Embedded Engine - Route AI requests seamlessly
AppContact=https://github.com/Finesssee/ProxyPilot

; --- Installation Paths ---
DefaultDirName={localappdata}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible

; --- Compression ---
Compression=lzma2/ultra64
SolidCompression=yes

; --- Installer Appearance ---
WizardStyle=modern
WizardSizePercent=100
WizardResizable=no

; --- Custom Branding Images ---
; WizardImageFile: Left panel image (164x314 pixels for modern style)
; WizardSmallImageFile: Top-right corner image (55x55 pixels)
WizardImageFile={#RepoRoot}\installer\assets\wizard-large.bmp
WizardSmallImageFile={#RepoRoot}\installer\assets\wizard-small.bmp

; --- Icons ---
SetupIconFile={#RepoRoot}\static\icon.ico
UninstallDisplayIcon={app}\icon.ico

; --- Output ---
OutputDir={#OutDir}
OutputBaseFilename=ProxyPilot-{#AppVersion}-Setup

; --- Installer Window Title ---
AppCopyright=MIT License - ProxyPilot Contributors

; --- Version Info Embedded in Installer ---
VersionInfoVersion={#AppVersion}
VersionInfoCompany=ProxyPilot
VersionInfoDescription=ProxyPilot Installer - Local AI Proxy
VersionInfoTextVersion={#AppVersion}
VersionInfoCopyright=MIT License
VersionInfoProductName=ProxyPilot
VersionInfoProductVersion={#AppVersion}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Messages]
; --- Custom Welcome Page Text ---
WelcomeLabel1=Welcome to ProxyPilot
WelcomeLabel2=This will install [name/ver] on your computer.%n%nProxyPilot is a local AI proxy that routes requests to Claude, OpenAI, and other providers with automatic credential management.%n%nIt is recommended that you close all other applications before continuing.

; --- Custom Finished Page Text ---
FinishedHeadingLabel=ProxyPilot Installation Complete
FinishedLabelNoIcons=Setup has finished installing [name] on your computer.
FinishedLabel=Setup has finished installing [name] on your computer. ProxyPilot will appear in your system tray.

; --- Custom Ready Page ---
ReadyLabel1=Ready to Install
ReadyLabel2a=Click Install to begin the ProxyPilot installation.
ReadyLabel2b=Click Install to begin the ProxyPilot installation. Review your settings below:

; --- Custom Select Dir Page ---
SelectDirLabel3=ProxyPilot will be installed in the following folder.
SelectDirBrowseLabel=To continue, click Next. To select a different folder, click Browse.

; --- Custom Buttons (optional - uncomment to customize) ---
; ButtonNext=&Continue
; ButtonInstall=&Install ProxyPilot
; ButtonFinish=&Launch ProxyPilot

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked
Name: "startupicon"; Description: "Launch ProxyPilot when Windows starts"; GroupDescription: "Startup options:"; Flags: checkedonce

[Files]
; Main app (tray + dashboard UI)
Source: "{#OutDir}\ProxyPilot.exe"; DestDir: "{app}"; Flags: ignoreversion

; Config
Source: "{#RepoRoot}\config.example.yaml"; DestDir: "{app}"; Flags: ignoreversion

; Icons and branding
Source: "{#RepoRoot}\static\icon.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\static\icon.png"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\ProxyPilot.exe"; WorkingDir: "{app}"; IconFilename: "{app}\icon.ico"; Comment: "Launch ProxyPilot - Local AI Proxy"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\ProxyPilot.exe"; Tasks: desktopicon; WorkingDir: "{app}"; IconFilename: "{app}\icon.ico"; Comment: "Launch ProxyPilot"

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "ProxyPilot"; ValueData: """{app}\ProxyPilot.exe"""; Flags: uninsdeletevalue; Tasks: startupicon

[Run]
Filename: "{app}\ProxyPilot.exe"; Description: "Launch {#AppName} now"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "taskkill"; Parameters: "/F /IM ProxyPilot.exe"; Flags: runhidden; RunOnceId: "KillTray"

[UninstallDelete]
Type: filesandordirs; Name: "{app}"

[Code]
// Custom colors and initialization
procedure InitializeWizard();
begin
  // Set custom header color (ProxyPilot blue theme)
  WizardForm.Color := $00362B21;  // Dark background

  // Customize the main panel if desired
  // WizardForm.MainPanel.Color := $00362B21;

  // Make the installer window title more branded
  WizardForm.Caption := 'ProxyPilot Setup';
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ConfigYaml: string;
  ExampleYaml: string;
begin
  if CurStep = ssPostInstall then
  begin
    // Auto-create config.yaml from example if it doesn't exist
    ConfigYaml := ExpandConstant('{app}\config.yaml');
    ExampleYaml := ExpandConstant('{app}\config.example.yaml');
    if (not FileExists(ConfigYaml)) and FileExists(ExampleYaml) then
    begin
      CopyFile(ExampleYaml, ConfigYaml, False);
    end;
  end;
end;

// Custom uninstall confirmation message
function InitializeUninstall(): Boolean;
begin
  Result := MsgBox('Are you sure you want to uninstall ProxyPilot?' + #13#10 + #13#10 +
                   'Your configuration files will be preserved.',
                   mbConfirmation, MB_YESNO) = IDYES;
end;
