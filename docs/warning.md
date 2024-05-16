# Caution and Warning

!!! warning
    **We strongly advise you to not run this application on any GCP project, where you cannot afford to lose
    all resources.**

To reduce the blast radius of accidents, there are some safety precautions:

1. By default, **gcp-nuke** only lists all nuke-able resources. You need to add `--no-dry-run` to actually delete
   resources.
2. **gcp-nuke** asks you twice to confirm the deletion by entering the project alias. The first time is directly
   after the start and the second time after listing all nuke-able resources.
       
    !!! note "ProTip"
        This can be disabled by adding `--no-prompt` to the command line. 

3. The config file contains a blocklist field. If the Project ID of the project you want to nuke is part of this
   blocklist, **gcp-nuke** will abort. It is recommended, that you add every production project to this blocklist.
4. To ensure you don't just ignore the blocklisting feature, the blocklist must contain at least one Project ID.
5. The config file contains project specific settings (e.g. filters). The project you want to nuke must be explicitly
   listed there.
6. To ensure to not accidentally delete a random project, it is required to specify a config file. It is recommended
   to have only a single config file and add it to a central repository. This way the blocklist is easier to manage and
   keep up to date.

Feel free to create an issue, if you have any ideas to improve the safety procedures.

