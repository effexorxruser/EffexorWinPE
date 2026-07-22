# Driver staging

Put locally obtained, reviewed driver packages in `drivers/local/<set-name>/`. That directory is ignored by Git and copied into the mounted image only when explicitly enabled by the build script.

Do not import an installed Windows DriverStore wholesale. Keep storage and wired-network coverage focused, versioned, and auditable. Wi-Fi is not an MVP dependency.
