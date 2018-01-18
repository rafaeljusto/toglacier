package main

var webHomepageTemplate = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>toglacier</title>

    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-beta.3/css/bootstrap.min.css" integrity="sha384-Zug+QiDoJOrZ5t4lssLdxGhVrurbmBWopoEl+M6BdEfwnCJZtKxi1KgxUyJq13dy" crossorigin="anonymous">
  </head>
  <body>
    <nav class="navbar navbar-dark bg-primary">
      <a class="navbar-brand" href="#">toglacier</a>
    </nav>
    
    <div class="container-fluid">
      <div class="row">
        <div class="col">
          <div class="alert alert-danger alert-dismissible fade show mt-2" role="alert">
            <button type="button" class="close" data-dismiss="alert" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
            <p></p>
          </div>
        </div>
      </div>

      <div class="row">
        <div class="col">
          <ul class="nav nav-tabs mt-3 mb-3" role="tablist">
            <li class="nav-item">
              <a class="nav-link active" data-toggle="tab" href="#backups" role="tab">Backups</a>
            </li>
            <li class="nav-item">
              <a class="nav-link" data-toggle="tab" href="#settings" role="tab">Settings</a>
            </li>
          </ul>
        </div>
      </div>

      <div class="row">
        <div class="col">
          <div class="tab-content">
            <div class="tab-pane fade show active" id="backups" role="tabpanel" aria-labelledby="backups-tab">
              <table class="table">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>Date</th>
                  </tr>
                </thead>
                <tbody id="backups-results"></tbody>
              </table>
            </div>
            <div class="tab-pane fade" id="settings" role="tabpanel" aria-labelledby="settings-tab">
              <form>
                <div class="form-group">
                  <label for="paths">Paths</label>
                  <textarea id="paths" class="form-control" aria-describedby="pathsHelp" placeholder="Enter paths"></textarea>
                  <small id="pathsHelp" class="form-text text-muted">
                    Paths is the list of all locations that you want to backup.
                    It could be a directory or a specific file. Add one path per
                    line.
                  </small>
                </div>
                <div class="form-group">
                  <label for="keep-backups">Keep Backups</label>
                  <input id="keep-backups" type="number" min="0" class="form-control" aria-describedby="keepBackupsHelp" placeholder="Enter the number of backups to keep" />
                  <small id="keepBackupsHelp" class="form-text text-muted">
                    Keep backups defines the number of recent backups to
                    preserve (by creation date). The idea is to remove older
                    backups so we don't spent too much space in the cloud. All
                    dependent backups (incremental parts) are also kept so you
                    can rebuild successfully. By default we will keep the last
                    10 backups.
                  </small>
                </div>
                <div class="form-group">
                  <label for="backup-secret">Backup secret</label>
                  <input id="backup-secret" type="password" class="form-control" aria-describedby="backupSecretHelp" placeholder="Enter the backup secret" />
                  <small id="backupSecretHelp" class="form-text text-muted">
                    Backup secret is an optional parameter that increase the
                    security of your backup in the cloud. If a passphrase is
                    informed the backup tarball is encrypted (OFB) and signed
                    (HMAC256). You will need to have the same passphrase when
                    retrieving an encrypted backup. The passphrase can be
                    encrypted with the 'toglacier encrypt' command to avoid
                    having it in plain text.
                  </small>
                </div>
                <div class="form-group">
                  <label for="modify-tolerance">Modify tolerance</label>
                  <input id="modify-tolerance" type="number" min="0" max="100" class="form-control" aria-describedby="modifyToleranceHelp" placeholder="Enter the modify tolerance" />
                  <small id="modifyToleranceHelp" class="form-text text-muted">
                    Modify tolerance defines the percentage of modified files
                    that can be tolerated between two backups. This is important
                    to detect ransomware infections, when all files in disk are
                    encrypted by a computer virus. This value should be defined
                    according to your behavior, if you usually modify all files
                    in the backup folders this percentage should be high,
                    otherwise you can decrease the value to a safety line.
                    Values 0% or 100% disables this check. By default is 0%.
                  </small>
                </div>
                <div class="form-group">
                  <label for="ignore-patterns">Ignore patterns</label>
                  <textarea id="ignore-patterns" class="form-control" aria-describedby="ignorePatternsHelp" placeholder="Enter the regular expression"></textarea>
                  <small id="ignorePatternsHelp" class="form-text text-muted">
                    Ignore patterns removes from the backup files that matches
                    one or more patterns of this list. This is useful to avoid
                    temporary or lock files in your backup.
                  </small>
                </div>
                <div class="form-group">
                  <label>Cloud</label>
                  <div class="form-check">
                    <label class="form-check-label">
                      <input id="cloud-aws" name="cloud" type="radio" class="form-check-input" aria-describedby="cloudHelp" value="aws" />
                      Amazon Web Services
                    </label>
                  </div>
                  <div class="form-check">
                    <label class="form-check-label">
                      <input id="cloud-gcs" name="cloud" type="radio" class="form-check-input" aria-describedby="cloudHelp" value="gcs" />
                      Google Cloud Storage
                    </label>
                  </div>
                  <small id="cloudHelp" class="form-text text-muted">
                    Cloud determinates the cloud service will be used to manage
                    the backups. The possible values are aws or gcs. By default
                    aws will be used.
                  </small>
                </div>
              </form>
            </div>
          </div>
        </div>
      </div>
    </div>

    <script src="https://code.jquery.com/jquery-3.2.1.min.js" integrity="sha256-hwg4gsxgFZhOsEEamdOYGBf13FyQuiTwlAQgxVSNgt4=" crossorigin="anonymous"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/popper.js/1.12.9/umd/popper.min.js" integrity="sha384-ApNbgh9B+Y1QKtv3Rn7W3mgPxhU9K/ScQsAP7hUibX39j7fakFPskvXusvfa0b4Q" crossorigin="anonymous"></script>
    <script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-beta.3/js/bootstrap.min.js" integrity="sha384-a5N7Y/aK3qNeh15eJKGWxsqtnX/wWdSZSKp+81YjTmS15nvnvxKHuzaWwXHDli+4" crossorigin="anonymous"></script>

    <script type="text/javascript">
      function backups() {
        $.ajax("/backups")

          .done(function(backups, textStatus, jqXHR) {
            $("#backups-results").empty();
            $.each(JSON.parse(backups), function(index, backup) {
              $("#backups-results").append(
                $("<tr>").append(
                  $("<th>")
                    .attr("scope", "row")
                    .text(index),
                  $("<td>").text(backup.Backup.CreatedAt)
                )
              );
            });
          })

          .fail(function(jqXHR, textStatus, errorThrown) {
            $(".alert-danger > p").text("Error loading backups");
            $(".alert").alert();
          })

          .always(function() {
            $(".alert-danger > p").text("");
            $(".alert").alert("close");
            setTimeout(backups, 5000);
          });
      }

      function config() {
        $.ajax("/config")

          .done(function(data, textStatus, jqXHR) {
            var config = JSON.parse(data);

            $("#paths").val("");
            $.each(config.paths, function(index, path) {
              $("#paths").val(path + "\n");
            });
            $("#paths").val($.trim($("#paths").val()));

            $("#keep-backups").val(config.keepBackups);
            $("#backup-secret").val(config.backupSecret);
            $("#modify-tolerance").val(config.modifyTolerance);

            $("#ignore-patterns").val("");
            $.each(config.ignorePatterns, function(index, ignorePattern) {
              $("#ignore-patterns").val(ignorePattern + "\n");
            });
            $("#ignore-patterns").val($.trim($("#ignore-patterns").val()));

            if (config.cloud == "aws") {
              $("#cloud-aws").prop("checked", true);
            } else if (config.cloud == "gcs") {
              $("#cloud-gcs").prop("checked", true);
            }
          })

          .fail(function(jqXHR, textStatus, errorThrown) {
            $(".alert-danger > p").text("Error loading config");
            $(".alert").alert();
          })

          .always(function() {
            $(".alert-danger > p").text("");
            $(".alert").alert("close");
            setTimeout(backups, 5000);
          });
      }

      $(function() {
        backups();
        config();
      });
    </script>
  </body>
</html>
`
